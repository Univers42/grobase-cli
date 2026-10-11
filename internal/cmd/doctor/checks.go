package doctor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/httpx"
	"github.com/Univers42/grobase-cli/internal/secret"
)

// check is one row of the report.
type check struct {
	name   string
	result string
	detail string
}

// run carries what the checks need. The clients are nil when no context is
// selected: every check then says so instead of panicking.
type run struct {
	ctx    context.Context
	e      *env.Env
	conf   config.Context
	plain  *httpx.Client // probes that send no credential
	authed *httpx.Client // the auth check only; nil when the store cannot be opened
}

// newRun resolves the current context and the clients the checks use. Building
// the plain client even when the credential store is unreadable is what keeps
// the connectivity checks working on a machine that never logged in.
func newRun(ctx context.Context, e *env.Env) *run {
	r := &run{ctx: ctx, e: e}
	c, err := e.Current()
	if err != nil {
		return r
	}
	r.conf = c
	r.plain, _ = e.HTTPClient(c, false)
	r.authed, _, _ = e.Client()
	return r
}

// answer is what one plain GET answered: the status line, the Date header and
// how long the round trip took. err is set when no answer arrived at all.
type answer struct {
	status string
	date   string
	ms     int64
	err    error
}

// get issues an unauthenticated GET through the client and reads only the
// headers: a 404 or a 401 is an answer, and its Date header is what the clock
// check needs.
func (r *run) get(cl *httpx.Client, path string) answer {
	start := time.Now()
	a := answer{}
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, strings.TrimSuffix(cl.Base.String(), "/")+path, nil)
	if err != nil {
		a.err = err
		return a
	}
	resp, err := cl.HTTP.Do(req)
	a.ms = time.Since(start).Milliseconds()
	if err != nil {
		a.err = err
		return a
	}
	defer func() { _ = resp.Body.Close() }()
	a.status, a.date = resp.Status, resp.Header.Get("Date")
	return a
}

// 1. The configuration: a context must be selected and valid.
func checkConfig(r *run) []check {
	c, err := r.e.Current()
	if err != nil {
		return row("config", resultFail, err.Error())
	}
	if verr := c.Validate(); verr != nil {
		return row("config", resultFail, fmt.Sprintf("context %q is invalid: %v: fix it with `grobase context add`", c.Name, verr))
	}
	return row("config", resultPass, fmt.Sprintf("context %q -> %s", c.Name, c.URL))
}

// 2. DNS and reachability: the base URL must answer HTTP at all.
func checkReach(r *run) []check {
	if r.plain == nil {
		return skipped("dns/reachability")
	}
	a := r.get(r.plain, "/")
	if a.err != nil {
		return row("dns/reachability", resultFail, a.err.Error())
	}
	return row("dns/reachability", resultPass, fmt.Sprintf("%s in %dms", a.status, a.ms))
}

// 3. TLS: how the context trusts the certificate it is presented.
func checkTLS(r *run) []check {
	if r.plain == nil {
		return skipped("tls")
	}
	u, err := url.Parse(r.conf.URL)
	if err != nil {
		return row("tls", resultFail, fmt.Sprintf("invalid url %q: %v", r.conf.URL, err))
	}
	if u.Scheme != "https" {
		return row("tls", resultWarn, "plaintext http: traffic can be read and altered on the path")
	}
	info, err := httpx.Probe(r.ctx, hostPort(u))
	if err != nil {
		return row("tls", resultFail, err.Error())
	}
	res, detail := tlsVerdict(r.conf, info, r.e.Clock())
	return row("tls", res, detail)
}

// 4. Clock skew: a wrong clock breaks token validation and log ordering.
func checkSkew(r *run) []check {
	if r.plain == nil {
		return skipped("clock skew")
	}
	a := r.get(r.plain, "/")
	if a.err != nil {
		return row("clock skew", resultFail, a.err.Error())
	}
	stamp, err := http.ParseTime(a.date)
	if err != nil {
		return row("clock skew", resultWarn, "the answer carries no usable Date header")
	}
	d := stamp.Sub(r.e.Clock())
	where := "behind"
	if d < 0 {
		where = "ahead"
	}
	return row("clock skew", skewVerdict(d),
		fmt.Sprintf("server clock is %+.0fs away (local clock is %s), %dms round trip", d.Seconds(), where, a.ms))
}

// 5. Credentials: which kinds are stored, never their values.
func checkCreds(r *run) []check {
	if r.plain == nil {
		return skipped("credentials")
	}
	kinds, err := storedKinds(r.e, r.conf.Name)
	if err != nil {
		return row("credentials", resultWarn, err.Error())
	}
	if len(kinds) == 0 {
		return row("credentials", resultWarn, "nothing stored: run `grobase login` or `grobase keys create`")
	}
	return row("credentials", resultPass, "stored: "+strings.Join(kinds, ", "))
}

// 6. Auth: the deployment must accept the stored credential.
func checkAuth(r *run) []check {
	if r.plain == nil {
		return skipped("auth")
	}
	kinds, err := storedKinds(r.e, r.conf.Name)
	switch {
	case err != nil:
		return row("auth", resultSkip, err.Error())
	case len(kinds) == 0:
		return row("auth", resultSkip, "no credential stored: nothing to check")
	}
	return authVerdict(r, kinds)
}

func authVerdict(r *run, kinds []string) []check {
	// A stored session is checked as itself; any other credential is enough for
	// the tenant route, which every authenticated plan answers.
	path, mode := "/v1/tenants/me", httpx.ModeAuto
	if slices.Contains(kinds, secret.KindSession) {
		path, mode = "/auth/v1/user", httpx.ModeUser
	}
	if r.authed == nil {
		return row("auth", resultSkip, "the credential store could not be opened, so no request was sent")
	}
	err := r.authed.JSON(r.ctx, httpx.Request{Method: http.MethodGet, Path: path, Mode: mode}, nil)
	if err == nil {
		return row("auth", resultPass, path+" answered 200")
	}
	var apiErr *httpx.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Unauthorized():
			return row("auth", resultFail, fmt.Sprintf("%s answered %d: the credential was rejected: run `grobase login`", path, apiErr.Status))
		case apiErr.NotFound():
			return row("auth", resultWarn, path+" is not enabled on this deployment, so the credential could not be proven")
		}
	}
	return row("auth", resultFail, err.Error())
}

// 7. Feature flags: what this deployment carries.
func checkFeatures(r *run) []check {
	if r.plain == nil {
		return skipped("features")
	}
	out := []check{}
	for _, path := range featurePaths() {
		out = append(out, feature(r, path))
	}
	return out
}

// featurePaths are the flag-gated routes that say what the deployment serves.
func featurePaths() []string {
	return []string{"/v1/orgs", "/query/v1/engines", "/functions/v1/", "/realtime/v1/"}
}

// feature probes one flag-gated route without a credential. Both outcomes are a
// PASS, the answer being the detail; only a probe that could not be made fails.
func feature(r *run, path string) check {
	name := "feature " + path
	resp, err := r.plain.Do(r.ctx, httpx.Request{Method: http.MethodGet, Path: path, Mode: httpx.ModeNone})
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		var apiErr *httpx.APIError
		if errors.As(err, &apiErr) && apiErr.NotFound() {
			return check{name: name, result: resultPass, detail: "not enabled on this deployment"}
		}
		if !errors.As(err, &apiErr) {
			return check{name: name, result: resultFail, detail: err.Error()}
		}
	}
	return check{name: name, result: resultPass, detail: "enabled"}
}

// storedKinds lists the credential kinds held for a context. Names only: a value
// never crosses this boundary, so a report cannot leak one.
func storedKinds(e *env.Env, ctxName string) ([]string, error) {
	store, err := e.Secrets()
	if err != nil {
		return nil, fmt.Errorf("credential store unavailable: %w", err)
	}
	var out []string
	for _, k := range []string{secret.KindAnon, secret.KindServiceRole, secret.KindAPIKey, secret.KindSession} {
		if v, gerr := store.Get(secret.Name(ctxName, k)); gerr == nil && v != "" {
			out = append(out, k)
		}
	}
	return out, nil
}

func row(name, result, detail string) []check {
	return []check{{name: name, result: result, detail: detail}}
}

func skipped(name string) []check {
	return row(name, resultSkip, "no context selected: run `grobase context add`")
}

// hostPort is the TLS endpoint of a URL, 443 by default.
func hostPort(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	return net.JoinHostPort(u.Hostname(), "443")
}
