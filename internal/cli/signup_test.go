package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Univers42/grobase-cli/internal/secret"
)

const (
	signupPassword = "pw-123456"
	signupJWT      = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1OSJ9.c2lnbmF0dXJlNDU2"
)

type signupReq struct {
	method, path, apikey, body string
}

// signupDeploy is a GoTrue deployment behind Kong. With autoConfirm on it answers
// /auth/v1/signup with a session (GOTRUE_MAILER_AUTOCONFIRM=true); otherwise with
// the bare user object GoTrue returns when the address must be confirmed.
type signupDeploy struct {
	mu          sync.Mutex
	reqs        []signupReq
	autoConfirm bool
}

func (d *signupDeploy) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	d.mu.Lock()
	d.reqs = append(d.reqs, signupReq{r.Method, r.URL.Path, r.Header.Get("apikey"), string(b)})
	d.mu.Unlock()
	switch r.URL.Path {
	case "/auth/v1/signup":
		if !d.autoConfirm {
			_, _ = rw.Write([]byte(`{"id":"u9","aud":"authenticated","email":"new@example.com"}`))
			return
		}
		_, _ = rw.Write([]byte(`{"access_token":"` + signupJWT + `","refresh_token":"signup-refresh-1",` +
			`"expires_in":3600,"user":{"id":"u9","email":"new@example.com"}}`))
	case "/auth/v1/user":
		_, _ = rw.Write([]byte(`{"id":"u9","email":"new@example.com","role":"authenticated"}`))
	default:
		http.NotFound(rw, r)
	}
}

// requests returns every call the deployment has seen so far.
func (d *signupDeploy) requests() []signupReq {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]signupReq, len(d.reqs))
	copy(out, d.reqs)
	d.reqs = nil
	return out
}

func newSignupWorld(t *testing.T, autoConfirm bool) (*world, *signupDeploy) {
	t.Helper()
	d := &signupDeploy{autoConfirm: autoConfirm}
	w := newWorld(t, d, anonKey+"\n"+signupPassword+"\n")
	d.requests() // drop whatever the harness sent while wiring the context
	return w, d
}

func TestSignupAutoConfirmStoresSession(t *testing.T) {
	w, d := newSignupWorld(t, true)
	if code := w.run("signup", "--email", "new@example.com", "--anon-key-stdin", "--password-stdin"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.errb)
	}
	reqs := d.requests()
	if len(reqs) != 1 || reqs[0].method != "POST" || reqs[0].path != "/auth/v1/signup" || reqs[0].apikey != anonKey {
		t.Fatalf("requests = %+v", reqs)
	}
	if want := `{"email":"new@example.com","password":"` + signupPassword + `"}`; reqs[0].body != want {
		t.Fatalf("body = %s, want %s", reqs[0].body, want)
	}
	if v, err := w.store.Get(secret.Name("t", secret.KindSession)); err != nil || !strings.Contains(v, "signup-refresh-1") {
		t.Fatalf("session not stored: %v", err)
	}
	if !strings.Contains(w.out.String(), `signed up and logged in to "t" as new@example.com`) {
		t.Fatalf("stdout = %q", w.out)
	}
	w.out.Reset()
	w.errb.Reset()
	if code := w.run("whoami", "-o", "json"); code != 0 {
		t.Fatalf("whoami exit %d: %s", code, w.errb)
	}
	var got map[string]any
	if err := json.Unmarshal(w.out.Bytes(), &got); err != nil {
		t.Fatalf("whoami output not JSON: %v\n%s", err, w.out)
	}
	if u, _ := got["user"].(map[string]any); u["email"] != "new@example.com" || got["session"] != true {
		t.Fatalf("whoami = %v", got)
	}
}

func TestSignupConfirmationRequired(t *testing.T) {
	w, d := newSignupWorld(t, false)
	if code := w.run("signup", "--email", "new@example.com", "--anon-key-stdin", "--password-stdin"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.errb)
	}
	if len(d.requests()) != 1 {
		t.Fatal("expected exactly one signup call")
	}
	want := "account created for new@example.com; confirm the email, then: grobase login --email new@example.com"
	if !strings.Contains(w.out.String(), want) {
		t.Fatalf("stdout = %q", w.out)
	}
	if _, err := w.store.Get(secret.Name("t", secret.KindSession)); err == nil {
		t.Fatal("no session may be stored before the email is confirmed")
	}
}

func TestSignupWithoutEmailIsAUsageError(t *testing.T) {
	w, d := newSignupWorld(t, true)
	if code := w.run("signup", "--anon-key-stdin", "--password-stdin"); code != 2 {
		t.Fatalf("exit %d, want 2 (stderr: %s)", code, w.errb)
	}
	if reqs := d.requests(); len(reqs) != 0 {
		t.Fatalf("no request may be sent without --email, got %+v", reqs)
	}
}

func TestSignupExistingEmailFails(t *testing.T) {
	w := newWorld(t, http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = rw.Write([]byte(`{"msg":"User already registered"}`))
	}), anonKey+"\n"+signupPassword+"\n")
	if code := w.run("signup", "--email", "new@example.com", "--anon-key-stdin", "--password-stdin"); code == 0 {
		t.Fatalf("exit 0, want a failure (stdout: %q)", w.out)
	}
	if !strings.Contains(w.errb.String(), "already registered") {
		t.Fatalf("stderr = %q", w.errb)
	}
	if _, err := w.store.Get(secret.Name("t", secret.KindSession)); err == nil {
		t.Fatal("a failed signup must not store a session")
	}
}

func TestSignupDebugNeverPrintsCredentials(t *testing.T) {
	w, _ := newSignupWorld(t, true)
	if code := w.run("--debug", "signup", "--email", "new@example.com", "--anon-key-stdin", "--password-stdin"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.errb)
	}
	all := w.out.String() + w.errb.String()
	for _, leak := range []string{signupPassword, anonKey, signupJWT, "signup-refresh-1"} {
		if strings.Contains(all, leak) {
			t.Errorf("output leaks %q:\n%s", leak, all)
		}
	}
	if !strings.Contains(w.errb.String(), "/auth/v1/signup") {
		t.Fatalf("expected a request trace on stderr, got:\n%s", w.errb)
	}
}
