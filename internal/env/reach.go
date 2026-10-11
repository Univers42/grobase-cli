package env

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// candidates returns the URLs to try in order: the primary URL first.
func candidates(c config.Context) []string {
	out := make([]string, 0, len(c.Alternates)+1)
	out = append(out, c.URL)
	out = append(out, c.Alternates...)
	return out
}

// probeTimeout returns the per-URL probe timeout from the environment.
func probeTimeout(e *Env) time.Duration {
	if s := e.Getenv("GROBASE_PROBE_TIMEOUT"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			return d
		}
	}
	return 2 * time.Second
}

// probeOne reports whether candidate answers an unauthenticated health check.
// ponytail: reachable is not authorized; a URL that answers but routes to a different deployment is still picked (the CA/pin check is what refuses it).
func probeOne(c config.Context, candidate string, timeout time.Duration) error {
	tr, err := httpx.NewTransport(httpx.TLS{CAFile: c.CAFile, PinSHA256: c.PinSHA256, Insecure: c.Insecure}, timeout)
	if err != nil {
		return err
	}
	cl := &http.Client{Transport: tr, Timeout: timeout}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(candidate, "/")+"/auth/v1/health", nil)
	if err != nil {
		return err
	}
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// cachedPick returns the memoized choice for a context name.
func cachedPick(e *Env, name string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.picked == nil {
		return "", false
	}
	u, ok := e.picked[name]
	return u, ok
}

// storePick memoizes the chosen URL for a context name.
func storePick(e *Env, name, chosen string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.picked == nil {
		e.picked = map[string]string{}
	}
	e.picked[name] = chosen
}

// debugUsing prints the chosen URL and the skipped candidates to stderr.
func (e *Env) debugUsing(name, chosen, skipped string) {
	if skipped == "" {
		_, _ = fmt.Fprintf(e.Stderr, "context %s: using %s\n", name, chosen)
		return
	}
	_, _ = fmt.Fprintf(e.Stderr, "context %s: using %s (skipped %s)\n", name, chosen, skipped)
}

// probeAll returns the first reachable candidate or a joined failure error.
func probeAll(c config.Context, cands []string, timeout time.Duration) (string, []string, error) {
	fails := make([]string, 0, len(cands))
	for _, u := range cands {
		if err := probeOne(c, u, timeout); err == nil {
			return u, fails, nil
		} else {
			fails = append(fails, fmt.Sprintf("%s (%s)", u, err))
		}
	}
	return "", fails, fmt.Errorf("context %q: no URL answered: %s", c.Name, strings.Join(fails, ", "))
}

// SelectedURL picks the first reachable URL of a context and memoizes it.
func (e *Env) SelectedURL(c config.Context) (string, error) {
	if len(c.Alternates) == 0 {
		return c.URL, nil
	}
	if u, ok := cachedPick(e, c.Name); ok {
		if e.Flags.Debug {
			e.debugUsing(c.Name, u, "")
		}
		return u, nil
	}
	chosen, fails, err := probeAll(c, candidates(c), probeTimeout(e))
	if err != nil {
		return "", err
	}
	storePick(e, c.Name, chosen)
	if e.Flags.Debug {
		e.debugUsing(c.Name, chosen, strings.Join(fails, ", "))
	}
	return chosen, nil
}
