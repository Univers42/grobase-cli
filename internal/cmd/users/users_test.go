package users_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/users"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const (
	serviceKey = "svc-key-123456"
	// userJSON is the GoTrue admin view of one user.
	userJSON = `{"id":"u1","email":"a@b.c","role":"authenticated","created_at":"2026-01-02T03:04:05Z"}`
	// strongPassword passes the policy in policy.go.
	strongPassword = "Correct-Horse-9!"
)

// reply answers a request; the body is the parsed-off request payload.
type reply func(w http.ResponseWriter, r *http.Request, body string)

// captured is one request the fake server received.
type captured struct {
	method, path string
	query        url.Values
	body         string
	header       http.Header
}

// fake records every request and answers with reply.
type fake struct {
	reply reply
	mu    sync.Mutex
	reqs  []captured
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, captured{r.Method, r.URL.EscapedPath(), r.URL.Query(), string(body), r.Header.Clone()})
	f.mu.Unlock()
	f.reply(w, r, string(body))
}

func (f *fake) last(t *testing.T) captured {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.reqs) == 0 {
		t.Fatal("no request reached the server")
	}
	return f.reqs[len(f.reqs)-1]
}

func (f *fake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

// answerEvery replies the same body to every request.
func answerEvery(status int, body string) reply {
	return func(w http.ResponseWriter, _ *http.Request, _ string) {
		respond(w, status, body)
	}
}

func respond(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// newWorld starts a command test world: the service-role key is stored so that admin
// calls authenticate, and stdin is what the command reads from its standard input.
func newWorld(t *testing.T, stdin string, reply reply) (*testenv.Harness, *fake) {
	t.Helper()
	f := &fake{reply: reply}
	return testenv.New(t, f, map[string]string{secret.KindServiceRole: serviceKey}, stdin), f
}

// world is the common case: one canned answer, no stdin.
func world(t *testing.T) (*testenv.Harness, *fake) {
	t.Helper()
	return newWorld(t, "", answerEvery(200, userJSON))
}

func run(t *testing.T, h *testenv.Harness, args ...string) error {
	t.Helper()
	return h.Run(users.New(h.Env), args...)
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

// bodyOf decodes a captured request body. The body is never echoed: it may hold a password.
func bodyOf(t *testing.T, c captured) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(c.body), &m); err != nil {
		t.Fatalf("request body is not a JSON object: %v", err)
	}
	return m
}

func outOf(t *testing.T, h *testenv.Harness) []any {
	t.Helper()
	var v []any
	if err := json.Unmarshal(h.Out.Bytes(), &v); err != nil {
		t.Fatalf("stdout is not a JSON array: %v\n%s", err, h.Out)
	}
	return v
}

func outObject(t *testing.T, h *testenv.Harness) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(h.Out.Bytes(), &v); err != nil {
		t.Fatalf("stdout is not a JSON object: %v\n%s", err, h.Out)
	}
	return v
}

func TestListUsers(t *testing.T) {
	h, f := newWorld(t, "", answerEvery(200, `{"users":[`+userJSON+`],"total":1}`))
	mustRun(t, h, "list")
	c := f.last(t)
	if c.method != http.MethodGet {
		t.Errorf("method = %q, want GET", c.method)
	}
	if c.path != "/auth/v1/admin/users" {
		t.Errorf("path = %q", c.path)
	}
	if got := c.query.Get("page"); got != "1" {
		t.Errorf("page = %q, want 1", got)
	}
	if got := c.query.Get("per_page"); got != "50" {
		t.Errorf("per_page = %q, want 50", got)
	}
	if c.header.Get("apikey") != serviceKey {
		t.Error("admin calls must authenticate with the service-role key")
	}
	if want := "Bearer " + serviceKey; c.header.Get("Authorization") != want {
		t.Error("admin calls must send the service-role key as the bearer token")
	}
	rows := outOf(t, h)
	if len(rows) != 1 {
		t.Fatalf("printed %d users, want 1: %s", len(rows), h.Out)
	}
	row, _ := rows[0].(map[string]any)
	if row["email"] != "a@b.c" {
		t.Errorf("printed %v", rows[0])
	}
}

func TestListUsersTableColumns(t *testing.T) {
	h, _ := newWorld(t, "", answerEvery(200, `{"users":[`+userJSON+`,`+
		`{"id":"u2","email":"d@e.f","role":"anon","created_at":"2026-01-03T00:00:00Z","banned_until":"2026-02-01T00:00:00Z"}`+
		`],"total":2}`))
	h.Env.Flags.Output = "table"
	mustRun(t, h, "list")
	out := h.Out.String()
	for _, col := range []string{"ID", "EMAIL", "ROLE", "CREATED_AT", "BANNED_UNTIL"} {
		if !strings.Contains(out, col) {
			t.Errorf("table is missing the %s column:\n%s", col, out)
		}
	}
	if !strings.Contains(out, "2026-02-01T00:00:00Z") {
		t.Errorf("banned_until not rendered:\n%s", out)
	}
}

func TestListUsersPagination(t *testing.T) {
	h, f := world(t)
	mustRun(t, h, "list", "--page", "3", "--per-page", "7")
	if got := f.last(t).query.Get("page"); got != "3" {
		t.Errorf("page = %q, want 3", got)
	}
	if got := f.last(t).query.Get("per_page"); got != "7" {
		t.Errorf("per_page = %q, want 7", got)
	}
}

func TestListUsersRejectsZeroPaging(t *testing.T) {
	h, f := world(t)
	if got := exitCode(t, h, "list", "--page", "0"); got != exitcode.Usage {
		t.Errorf("exit %d, want 2", got)
	}
	if f.count() != 0 {
		t.Error("a broken --page must not reach the server")
	}
}

func TestGetUserByID(t *testing.T) {
	h, f := world(t)
	mustRun(t, h, "get", "u1")
	c := f.last(t)
	if c.method != http.MethodGet || c.path != "/auth/v1/admin/users/u1" {
		t.Errorf("request = %s %s", c.method, c.path)
	}
	if got := outObject(t, h); got["id"] != "u1" {
		t.Errorf("printed %v", got)
	}
}

func TestGetUserByEmailScansTheList(t *testing.T) {
	h, f := newWorld(t, "", answerEvery(200, `{"users":[`+userJSON+`],"total":1}`))
	mustRun(t, h, "get", "a@b.c")
	c := f.last(t)
	if c.path != "/auth/v1/admin/users" {
		t.Errorf("path = %q, want the user list", c.path)
	}
	if got := c.query.Get("per_page"); got != "1000" {
		t.Errorf("per_page = %q, want 1000", got)
	}
	if got := c.query.Get("page"); got != "" {
		t.Errorf("page = %q, want the unpaged list", got)
	}
	if got := outObject(t, h); got["email"] != "a@b.c" {
		t.Errorf("printed %v", got)
	}
}

func TestGetUserByEmailMatchesExactly(t *testing.T) {
	body := `{"users":[{"id":"u2","email":"other@b.c"},` + userJSON + `],"total":2}`
	h, _ := newWorld(t, "", answerEvery(200, body))
	mustRun(t, h, "get", "a@b.c")
	if got := outObject(t, h); got["id"] != "u1" {
		t.Errorf("printed %v, want u1", got)
	}
}

func TestGetUserNotFoundExitCodes(t *testing.T) {
	h, _ := newWorld(t, "", answerEvery(404, `{"msg":"User not found"}`))
	if got := exitCode(t, h, "get", "u404"); got != exitcode.NotFound {
		t.Errorf("exit %d, want 4", got)
	}
	h, _ = newWorld(t, "", answerEvery(200, `{"users":[],"total":0}`))
	if got := exitCode(t, h, "get", "ghost@b.c"); got != exitcode.NotFound {
		t.Errorf("unknown email: exit %d, want 4", got)
	}
}

func TestAdminCallsNeedTheServiceRoleKey(t *testing.T) {
	h := testenv.New(t, http.NotFoundHandler(), nil, "")
	if got := exitCode(t, h, "list"); got != exitcode.AuthFail {
		t.Errorf("exit %d, want 3 without a service-role key", got)
	}
}
