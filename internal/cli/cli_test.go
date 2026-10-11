package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Univers42/grobase-cli/internal/cli"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/secret"
)

const (
	anonKey = "anon-key-abcdef"
	jwtLike = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1MSJ9.c2lnbmF0dXJlMTIz"
)

type world struct {
	out, errb *bytes.Buffer
	store     *secret.Memory
	run       func(args ...string) int
}

// newWorld wires the real command tree (cli.NewRoot) to an httptest server and an
// in-memory credential store, so tests exercise flag parsing, exit codes and output.
func newWorld(t *testing.T, h http.Handler, stdin string) *world {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	w := &world{out: &bytes.Buffer{}, errb: &bytes.Buffer{}, store: secret.NewMemory()}
	cfgPath := t.TempDir() + "/config.json"
	w.run = func(args ...string) int {
		root, _ := cli.NewRoot(env.Deps{
			Stdin: strings.NewReader(stdin), Stdout: w.out, Stderr: w.errb,
			Clock:        func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
			Getenv:       func(string) string { return "" },
			ConfigPath:   cfgPath,
			Interactive:  func() bool { return false },
			ReadPassword: func(string) (string, error) { t.Fatal("prompt"); return "", nil },
			OpenSecrets:  func(secret.Options) (secret.Store, string, error) { return w.store, "memory", nil },
		}, cli.BuildInfo{Version: "1.2.3", Commit: "abc", Date: "d"})
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			_, _ = w.errb.WriteString("error: " + err.Error() + "\n")
			return cli.ExitCode(err)
		}
		return 0
	}
	if code := w.run("context", "add", "t", "--url", srv.URL); code != 0 {
		t.Fatalf("context add: exit %d: %s", code, w.errb)
	}
	w.out.Reset()
	w.errb.Reset()
	return w
}

func tokenHandler(status int) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/v1/token":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if status != 200 || body["password"] != "correct horse" {
				rw.WriteHeader(map[bool]int{true: status, false: 400}[status != 200])
				_, _ = rw.Write([]byte(`{"error_description":"Invalid login credentials"}`))
				return
			}
			_, _ = rw.Write([]byte(`{"access_token":"` + jwtLike + `","refresh_token":"refresh-secret-1","expires_in":3600,"user":{"id":"u1","email":"a@b.c"}}`))
		case "/auth/v1/user":
			_, _ = rw.Write([]byte(`{"id":"u1","email":"a@b.c","role":"authenticated"}`))
		case "/auth/v1/logout":
			rw.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(rw, r)
		}
	})
}

func TestLoginPasswordStoresEncryptedSessionAndWhoami(t *testing.T) {
	w := newWorld(t, tokenHandler(200), anonKey+"\ncorrect horse\n")
	if code := w.run("login", "--email", "a@b.c", "--anon-key-stdin", "--password-stdin"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.errb)
	}
	if v, err := w.store.Get(secret.Name("t", secret.KindSession)); err != nil || !strings.Contains(v, "refresh-secret-1") {
		t.Fatalf("session not stored: %v", err)
	}
	if strings.Contains(w.out.String()+w.errb.String(), "refresh-secret-1") {
		t.Fatal("login output leaked a token")
	}
	w.out.Reset()
	if code := w.run("whoami", "-o", "json"); code != 0 {
		t.Fatalf("whoami exit %d: %s", code, w.errb)
	}
	var got map[string]any
	if err := json.Unmarshal(w.out.Bytes(), &got); err != nil {
		t.Fatalf("whoami output not JSON: %v\n%s", err, w.out)
	}
	if u, _ := got["user"].(map[string]any); u["email"] != "a@b.c" || got["session"] != true {
		t.Fatalf("whoami = %v", got)
	}
}

func TestLoginFailuresMapToExitCodes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		h     http.Handler
		stdin string
		args  []string
		want  int
	}{
		{"wrong password is an auth failure", tokenHandler(401), anonKey + "\nnope\n", []string{"login", "--email", "a@b.c", "--anon-key-stdin", "--password-stdin"}, 3},
		{"no method is a usage error", tokenHandler(200), "", []string{"login"}, 2},
		{"email without anon key is a usage error", tokenHandler(200), "x\n", []string{"login", "--email", "a@b.c", "--password-stdin"}, 2},
		{"unknown flag is a usage error", tokenHandler(200), "", []string{"login", "--bogus"}, 2},
		{"unknown command is a usage error", tokenHandler(200), "", []string{"nonsense"}, 2},
		{"server rejects service key", http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(401) }), "svc-key-123456\n", []string{"login", "--service-role"}, 3},
		{"server error on key check is only a warning", http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(503) }), "svc-key-123456\n", []string{"login", "--service-role"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t, tc.h, tc.stdin)
			if got := w.run(tc.args...); got != tc.want {
				t.Fatalf("exit %d, want %d (stderr: %s)", got, tc.want, w.errb)
			}
		})
	}
}

func TestRejectedServiceKeyIsForgotten(t *testing.T) {
	w := newWorld(t, http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(403) }), "svc-key-123456\n")
	_ = w.run("login", "--service-role")
	if _, err := w.store.Get(secret.Name("t", secret.KindServiceRole)); err == nil {
		t.Fatal("a rejected key must not stay in the credential store")
	}
}

func TestLogoutAndTokenPrint(t *testing.T) {
	w := newWorld(t, tokenHandler(200), anonKey+"\ncorrect horse\n")
	if w.run("login", "--email", "a@b.c", "--anon-key-stdin", "--password-stdin") != 0 {
		t.Fatal(w.errb.String())
	}
	w.out.Reset()
	if code := w.run("token", "print", "--kind", "access"); code != 0 || !strings.Contains(w.out.String(), jwtLike) {
		t.Fatalf("token print: exit %d out=%q err=%s", code, w.out, w.errb)
	}
	if code := w.run("logout"); code != 0 {
		t.Fatalf("logout exit %d: %s", code, w.errb)
	}
	if _, err := w.store.Get(secret.Name("t", secret.KindSession)); err == nil {
		t.Fatal("session survived logout")
	}
	if _, err := w.store.Get(secret.Name("t", secret.KindAnon)); err != nil {
		t.Fatal("logout without --all must keep the anon key")
	}
	_ = w.run("logout", "--all")
	if _, err := w.store.Get(secret.Name("t", secret.KindAnon)); err == nil {
		t.Fatal("logout --all must drop every key")
	}
}

func TestDebugNeverPrintsCredentials(t *testing.T) {
	w := newWorld(t, tokenHandler(200), anonKey+"\ncorrect horse\n")
	if code := w.run("--debug", "login", "--email", "a@b.c", "--anon-key-stdin", "--password-stdin"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.errb)
	}
	_ = w.run("--debug", "whoami")
	all := w.out.String() + w.errb.String()
	for _, leak := range []string{anonKey, "correct horse", jwtLike, "refresh-secret-1", "c2lnbmF0dXJlMTIz"} {
		if strings.Contains(all, leak) {
			t.Errorf("--debug output leaks %q:\n%s", leak, all)
		}
	}
	if !strings.Contains(w.errb.String(), "POST") {
		t.Fatalf("expected a request trace on stderr, got:\n%s", w.errb)
	}
}

func TestContextCommands(t *testing.T) {
	w := newWorld(t, http.NotFoundHandler(), "")
	for _, tc := range []struct {
		args []string
		want int
	}{
		{[]string{"context", "list"}, 0},
		{[]string{"context", "show"}, 0},
		{[]string{"context", "add", "t", "--url", "http://x"}, 0},                                               // add replaces
		{[]string{"context", "add", "bad name", "--url", "http://x"}, 2},                                        // invalid name
		{[]string{"context", "add", "o", "--url", "https://x", "--ca-file", "/c", "--insecure-skip-verify"}, 2}, // conflicting trust
		{[]string{"context", "use", "ghost"}, 4},
		{[]string{"context", "remove", "ghost"}, 4},
		{[]string{"context", "remove", "t"}, 0},
		{[]string{"context", "show"}, 4},
	} {
		if got := w.run(tc.args...); got != tc.want {
			t.Errorf("%v: exit %d, want %d (stderr: %s)", tc.args, got, tc.want, w.errb)
		}
		w.errb.Reset()
	}
}

func TestInsecureContextWarnsLoudly(t *testing.T) {
	w := newWorld(t, http.NotFoundHandler(), "")
	if code := w.run("context", "add", "lab", "--url", "https://10.0.0.1", "--insecure-skip-verify"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.errb)
	}
	if !strings.Contains(strings.ToLower(w.errb.String()), "insecure") {
		t.Fatalf("no warning on stderr: %q", w.errb)
	}
}

func TestVersionAndHelpExitZero(t *testing.T) {
	w := newWorld(t, http.NotFoundHandler(), "")
	if code := w.run("version"); code != 0 || !strings.Contains(w.out.String(), "1.2.3") {
		t.Fatalf("version: exit %d out=%q", code, w.out)
	}
	if code := w.run("--help"); code != 0 {
		t.Fatalf("help: exit %d", code)
	}
}

func TestContextAddDedupsAlternates(t *testing.T) {
	a := httptest.NewServer(http.NotFoundHandler())
	defer a.Close()
	b := httptest.NewServer(http.NotFoundHandler())
	defer b.Close()
	w := newWorld(t, http.NotFoundHandler(), "")
	if code := w.run("context", "add", "dc", "--url", a.URL, "--url", b.URL, "--url", a.URL); code != 0 {
		t.Fatalf("add exit %d: %s", code, w.errb)
	}
	w.out.Reset()
	if code := w.run("context", "show", "dc", "-o", "json"); code != 0 {
		t.Fatalf("show exit %d: %s", code, w.errb)
	}
	var got map[string]any
	if err := json.Unmarshal(w.out.Bytes(), &got); err != nil {
		t.Fatalf("show not JSON: %v", err)
	}
	if got["url"] != a.URL {
		t.Fatalf("url = %v want %s", got["url"], a.URL)
	}
	alts, _ := got["alternates"].([]any)
	if len(alts) != 1 || alts[0] != b.URL {
		t.Fatalf("alternates = %v want [%s]", got["alternates"], b.URL)
	}
}

func TestContextEnvPrintsQuotedVars(t *testing.T) {
	w := newWorld(t, http.NotFoundHandler(), "")
	ca := t.TempDir() + "/a'b.pem"
	if err := os.WriteFile(ca, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if code := w.run("context", "add", "dc", "--url", srv.URL, "--ca-file", ca); code != 0 {
		t.Fatalf("add exit %d: %s", code, w.errb)
	}
	w.out.Reset()
	if code := w.run("context", "env", "dc"); code != 0 {
		t.Fatalf("env exit %d: %s", code, w.errb)
	}
	abs, _ := filepath.Abs(ca)
	want := "GROBASE_TARGET='" + srv.URL + "'\nGROBASE_CA_FILE='" + strings.ReplaceAll(abs, "'", "'\\''") + "'\n"
	if w.out.String() != want {
		t.Fatalf("env = %q want %q", w.out, want)
	}
	w.out.Reset()
	if code := w.run("context", "env", "t"); code != 0 {
		t.Fatalf("env t exit %d: %s", code, w.errb)
	}
	lines := strings.Split(strings.TrimSuffix(w.out.String(), "\n"), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "GROBASE_TARGET='") {
		t.Fatalf("env t = %q want one GROBASE_TARGET line", w.out)
	}
}

func TestContextEnvNoURLAnswered(t *testing.T) {
	s1 := httptest.NewServer(http.NotFoundHandler())
	s2 := httptest.NewServer(http.NotFoundHandler())
	u1, u2 := s1.URL, s2.URL
	s1.Close()
	s2.Close()
	w := newWorld(t, http.NotFoundHandler(), "")
	if code := w.run("context", "add", "dc", "--url", u1, "--url", u2); code != 0 {
		t.Fatalf("add exit %d: %s", code, w.errb)
	}
	w.out.Reset()
	w.errb.Reset()
	if code := w.run("context", "env", "dc"); code == 0 {
		t.Fatal("env with closed URLs must fail")
	}
	if !strings.Contains(w.errb.String(), "no URL answered") {
		t.Fatalf("stderr = %q want no URL answered", w.errb)
	}
}
