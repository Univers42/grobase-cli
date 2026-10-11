package apps_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Univers42/grobase-cli/internal/auth"
	"github.com/Univers42/grobase-cli/internal/cmd/apps"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const (
	anonKey = "anon-key-apps-123"
	userJWT = "user-jwt-apps-456"
	appsURL = "/v1/tenants/me/apps"
	// cleartextKey is the app key a fresh creation returns exactly once.
	cleartextKey = "mbk_CLEARTEXT_apps_789"
	appList      = `[{"app_id":"a1","name":"notes","status":"ready","created_at":"2026-01-02T03:04:05Z"},` +
		`{"app_id":"a2","name":"blog","status":"provisioning","created_at":"2026-01-03T00:00:00Z"}]`
	newApp = `{"tenant":"t_notes","db_id":"db_notes","status":"provisioning","kong_path":"/notes",` +
		`"api_key":"` + cleartextKey + `"}`
)

// captured is one request the fake server received.
type captured struct {
	method, path string
	body         string
	header       http.Header
}

// fake records every request and answers with reply.
type fake struct {
	reply func(w http.ResponseWriter, r *http.Request, body string)
	mu    sync.Mutex
	reqs  []captured
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, captured{r.Method, r.URL.EscapedPath(), string(body), r.Header.Clone()})
	f.mu.Unlock()
	f.reply(w, r, string(body))
}

// count is how many requests reached the server.
func (f *fake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

// last is the most recent request.
func (f *fake) last(t *testing.T) captured {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.reqs) == 0 {
		t.Fatal("no request reached the server")
	}
	return f.reqs[len(f.reqs)-1]
}

// answerEvery replies the same status and body to every request.
func answerEvery(status int, body string) func(http.ResponseWriter, *http.Request, string) {
	return func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// newWorld starts a command test world over reply with creds stored.
func newWorld(t *testing.T, creds map[string]string, reply func(http.ResponseWriter, *http.Request, string)) (*testenv.Harness, *fake) {
	t.Helper()
	f := &fake{reply: reply}
	return testenv.New(t, f, creds, ""), f
}

// userCreds is a logged-in user: the session authenticates the apps routes and
// the anon key rides along as apikey.
func userCreds(t *testing.T) map[string]string {
	t.Helper()
	s := auth.Session{
		AccessToken: userJWT, RefreshToken: "refresh-secret",
		ExpiresAt: time.Date(2026, 1, 2, 4, 4, 5, 0, time.UTC),
	}
	enc, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{secret.KindAnon: anonKey, secret.KindSession: enc}
}

func run(t *testing.T, h *testenv.Harness, args ...string) error {
	t.Helper()
	return h.Run(apps.New(h.Env), args...)
}

// mustRun fails the test when the command does not exit cleanly.
func mustRun(t *testing.T, h *testenv.Harness, args ...string) {
	t.Helper()
	if err := run(t, h, args...); err != nil {
		t.Fatalf("%v: %v (stderr: %s)", args, err, h.Err)
	}
}

// exitCode runs the command and returns the exit code the process would use.
func exitCode(t *testing.T, h *testenv.Harness, args ...string) int {
	t.Helper()
	return exitcode.Of(run(t, h, args...))
}

// rowsOf decodes stdout as a JSON array of apps.
func rowsOf(t *testing.T, h *testenv.Harness) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal(h.Out.Bytes(), &rows); err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, h.Out)
	}
	return rows
}

// objectOf decodes stdout as one JSON object.
func objectOf(t *testing.T, h *testenv.Harness) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(h.Out.Bytes(), &obj); err != nil {
		t.Fatalf("output is not a JSON object: %v\n%s", err, h.Out)
	}
	return obj
}

func TestAppsList(t *testing.T) {
	h, f := newWorld(t, userCreds(t), answerEvery(200, appList))
	mustRun(t, h, "list")
	c := f.last(t)
	if c.method != http.MethodGet || c.path != appsURL {
		t.Fatalf("got %s %s, want GET %s", c.method, c.path, appsURL)
	}
	if c.header.Get("apikey") != anonKey {
		t.Errorf("apikey = %q, want %q", c.header.Get("apikey"), anonKey)
	}
	if want := "Bearer " + userJWT; c.header.Get("Authorization") != want {
		t.Errorf("Authorization = %q, want %q", c.header.Get("Authorization"), want)
	}
	rows := rowsOf(t, h)
	if len(rows) != 2 || rows[0]["app_id"] != "a1" || rows[1]["status"] != "provisioning" {
		t.Fatalf("printed %v", rows)
	}
}

func TestAppsListTableColumns(t *testing.T) {
	h, _ := newWorld(t, userCreds(t), answerEvery(200, appList))
	h.Env.Flags.Output = "table"
	mustRun(t, h, "list")
	out := h.Out.String()
	for _, col := range []string{"APP_ID", "NAME", "STATUS", "CREATED_AT"} {
		if !strings.Contains(out, col) {
			t.Errorf("table is missing the %s column:\n%s", col, out)
		}
	}
}

func TestAppsListNotEnabled(t *testing.T) {
	h, _ := newWorld(t, userCreds(t), answerEvery(404, `{"msg":"no such route"}`))
	err := run(t, h, "list")
	if got := exitcode.Of(err); got != exitcode.NotFound {
		t.Fatalf("exit %d, want 4 (err=%v)", got, err)
	}
	if !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("err=%v, want the not-enabled wording", err)
	}
}

func TestAppsCreate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		wantBody string
	}{
		{"without an engine", []string{"create", "notes"}, `{"name":"notes"}`},
		{"with an engine", []string{"create", "notes", "--engine", "postgresql"}, `{"name":"notes","engine":"postgresql"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, f := newWorld(t, userCreds(t), answerEvery(201, newApp))
			mustRun(t, h, tc.args...)
			c := f.last(t)
			if c.method != http.MethodPost || c.path != appsURL {
				t.Fatalf("got %s %s, want POST %s", c.method, c.path, appsURL)
			}
			if c.body != tc.wantBody {
				t.Fatalf("body = %s, want %s", c.body, tc.wantBody)
			}
			if n := strings.Count(h.Out.String(), cleartextKey); n != 1 {
				t.Fatalf("cleartext key printed %d times, want once: %s", n, h.Out)
			}
			if !strings.Contains(h.Err.String(), "store this key now") {
				t.Fatalf("warning missing: %s", h.Err)
			}
		})
	}
}

func TestAppsCreateWithoutKeyDoesNotWarn(t *testing.T) {
	h, _ := newWorld(t, userCreds(t), answerEvery(201, `{"tenant":"t_blog","db_id":"db_blog","status":"provisioning"}`))
	mustRun(t, h, "create", "blog")
	if strings.Contains(h.Err.String(), "store this key") {
		t.Fatalf("no key was minted, so there is nothing to warn about: %s", h.Err)
	}
	if got := objectOf(t, h); got["db_id"] != "db_blog" {
		t.Fatalf("printed %v", got)
	}
}

func TestAppsCreateRequiresName(t *testing.T) {
	h, f := newWorld(t, userCreds(t), answerEvery(201, newApp))
	if got := exitCode(t, h, "create"); got != exitcode.Usage {
		t.Fatalf("exit %d, want 2", got)
	}
	if n := f.count(); n != 0 {
		t.Fatalf("%d requests reached the server", n)
	}
}

func TestAppsNeedALoggedInUser(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"list", []string{"list"}},
		{"create", []string{"create", "notes"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, f := newWorld(t, map[string]string{secret.KindAnon: anonKey}, answerEvery(201, newApp))
			if got := exitCode(t, h, tc.args...); got != exitcode.AuthFail {
				t.Fatalf("exit %d, want 3", got)
			}
			if n := f.count(); n != 0 {
				t.Fatalf("%d requests reached the server", n)
			}
		})
	}
}

func TestAppsDelete(t *testing.T) {
	t.Run("with --yes", func(t *testing.T) {
		h, f := newWorld(t, userCreds(t), answerEvery(204, ``))
		mustRun(t, h, "delete", "t_123", "--yes")
		c := f.last(t)
		if c.method != http.MethodDelete || c.path != appsURL+"/t_123" {
			t.Fatalf("got %s %s, want DELETE %s/t_123", c.method, c.path, appsURL)
		}
		if got := objectOf(t, h); got["deleted"] != "t_123" {
			t.Fatalf("printed %v", got)
		}
	})

	t.Run("refused without --yes", func(t *testing.T) {
		h, f := newWorld(t, userCreds(t), answerEvery(204, ``))
		if got := exitCode(t, h, "delete", "t_123"); got != exitcode.Usage {
			t.Fatalf("exit %d, want 2", got)
		}
		if n := f.count(); n != 0 {
			t.Fatalf("%d requests reached the server", n)
		}
	})

	t.Run("not found stays not found", func(t *testing.T) {
		h, _ := newWorld(t, userCreds(t), answerEvery(404, `{"msg":"unknown app"}`))
		err := run(t, h, "delete", "ghost", "--yes")
		if got := exitcode.Of(err); got != exitcode.NotFound {
			t.Fatalf("exit %d, want 4 (err=%v)", got, err)
		}
		if strings.Contains(err.Error(), "apps: not enabled") {
			t.Fatalf("err=%v, want a genuine not-found error, not the not-enabled relabelling", err)
		}
	})
}
