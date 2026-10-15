package db_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Univers42/grobase-cli/internal/cmd/db"
	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const (
	apiKey  = "mbk_test_api_key"
	connStr = "postgres://user:s3cret@db.example.com:5432/app"
	mounts  = "/v1/tenants/me/mounts"
)

// request is what a command actually put on the wire.
type request struct {
	Method, Path, Query string
	Body                map[string]any
	Auth, APIKey        string
	DataKey             string // X-Baas-Api-Key, the one credential /query/v1 reads
}

// recorder answers every call with status/body and keeps the last request.
type recorder struct {
	status int
	body   string
	hits   int
	last   request
}

func (r *recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.hits++
	r.last = request{
		Method: req.Method, Path: req.URL.Path, Query: req.URL.RawQuery,
		Auth: req.Header.Get("Authorization"), APIKey: req.Header.Get("apikey"),
		DataKey: req.Header.Get("X-Baas-Api-Key"),
	}
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		if len(bytes.TrimSpace(b)) > 0 {
			_ = json.Unmarshal(b, &r.last.Body)
		}
	}
	if r.status != 0 && r.status != http.StatusOK {
		w.WriteHeader(r.status)
	}
	_, _ = w.Write([]byte(r.body))
}

func newHarness(t *testing.T, rec *recorder, stdin string) *testenv.Harness {
	t.Helper()
	return testenv.New(t, rec, map[string]string{secret.KindAPIKey: apiKey}, stdin)
}

// customHarness fixes Getenv and the output format, two things testenv.New keeps
// hard-coded (empty env, json) — the $GROBASE_DB_CONNECTION_STRING path and the
// table columns both need them.
func customHarness(t *testing.T, rec *recorder, output string, getenv func(string) string, stdin string) *testenv.Harness {
	t.Helper()
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)
	h := &testenv.Harness{
		Server: srv, Store: secret.NewMemory(), Out: &bytes.Buffer{}, Err: &bytes.Buffer{},
		Stdin: strings.NewReader(stdin), Now: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	h.Env = env.New(env.Deps{
		Stdin: h.Stdin, Stdout: h.Out, Stderr: h.Err,
		Clock:        func() time.Time { return h.Now },
		Getenv:       getenv,
		ConfigPath:   t.TempDir() + "/config.json",
		Interactive:  func() bool { return false },
		ReadPassword: func(string) (string, error) { t.Fatal("prompt"); return "", nil },
		OpenSecrets:  func(secret.Options) (secret.Store, string, error) { return h.Store, "memory", nil },
		Version:      "test",
	}, &env.Flags{Output: output, Timeout: 10 * time.Second})
	cf := &config.File{}
	if err := cf.Add(config.Context{Name: testenv.ContextName, URL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if err := h.Env.ConfigStore().Save(cf); err != nil {
		t.Fatal(err)
	}
	if err := h.Store.Set(secret.Name(testenv.ContextName, secret.KindAPIKey), apiKey); err != nil {
		t.Fatal(err)
	}
	return h
}

func decodeOut(t *testing.T, h *testenv.Harness) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(h.Out.Bytes(), &v); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, h.Out)
	}
	return v
}

func run(t *testing.T, h *testenv.Harness, args ...string) int {
	t.Helper()
	return exitcode.Of(h.Run(db.New(h.Env), args...))
}

func TestDBListReadsBothAnswerShapes(t *testing.T) {
	row := map[string]any{
		"id": "m1", "name": "main", "engine": "postgresql",
		"isolation": "schema_per_tenant", "status": "ready",
	}
	for _, tc := range []struct {
		name string
		body string
	}{
		{"bare array", `[{"id":"m1","name":"main","engine":"postgresql","isolation":"schema_per_tenant","status":"ready"}]`},
		{"wrapped object", `{"mounts":[{"id":"m1","name":"main","engine":"postgresql","isolation":"schema_per_tenant","status":"ready"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{body: tc.body}
			h := newHarness(t, rec, "")
			if code := run(t, h, "list"); code != 0 {
				t.Fatalf("exit %d: %s", code, h.Err)
			}
			got, ok := decodeOut(t, h).([]any)
			if !ok || len(got) != 1 {
				t.Fatalf("list = %v", h.Out)
			}
			if diff := cmpJSON(got[0], row); diff != "" {
				t.Fatalf("mount row: %s", diff)
			}
			if rec.last.Method != http.MethodGet || rec.last.Path != mounts {
				t.Fatalf("request = %s %s", rec.last.Method, rec.last.Path)
			}
			if rec.last.Auth != "Bearer "+apiKey || rec.last.APIKey != "" {
				t.Fatalf("auth headers = %q / %q", rec.last.Auth, rec.last.APIKey)
			}
		})
	}
}

func TestDBTestReadsTheMountsTablesThroughTheQueryService(t *testing.T) {
	rec := &recorder{body: `{"engine":"mysql","tables":["projects","tasks"]}`}
	h := newHarness(t, rec, "")
	if code := run(t, h, "test", "m1"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.Err)
	}
	if rec.last.Method != http.MethodGet || rec.last.Path != "/query/v1/m1/tables" {
		t.Fatalf("request = %s %s", rec.last.Method, rec.last.Path)
	}
	if !strings.Contains(h.Out.String(), "projects") {
		t.Fatalf("output = %q", h.Out)
	}

	missing := &recorder{status: http.StatusNotFound, body: `{"message":"not found"}`}
	if code := run(t, newHarness(t, missing, ""), "test", "m1"); code == 0 {
		t.Fatal("a 404 from the query service exited 0")
	}
}

func TestDBListTableColumns(t *testing.T) {
	rec := &recorder{body: `[{"id":"m1","name":"main","engine":"postgresql","isolation":"shared","status":"ready"}]`}
	h := customHarness(t, rec, "table", func(string) string { return "" }, "")
	if code := run(t, h, "list"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.Err)
	}
	lines := strings.Split(strings.TrimSpace(h.Out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("table = %q", h.Out)
	}
	header, row := strings.Fields(lines[0]), strings.Fields(lines[1])
	for i, want := range []string{"ID", "NAME", "ENGINE", "ISOLATION", "STATUS"} {
		if header[i] != want {
			t.Fatalf("column %d = %q, want %q (header %q)", i, header[i], want, lines[0])
		}
	}
	for i, want := range []string{"m1", "main", "postgresql", "shared", "ready"} {
		if row[i] != want {
			t.Fatalf("cell %d = %q, want %q", i, row[i], want)
		}
	}
}

func TestDBAddSendsTheConnectionStringFromStdin(t *testing.T) {
	rec := &recorder{body: `{"id":"m1","status":"created","engine":"postgresql","name":"main"}`}
	h := newHarness(t, rec, connStr+"\n")
	if code := run(t, h, "add", "--name", "main", "--engine", "postgresql",
		"--isolation", "schema_per_tenant", "--connection-string-stdin"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.Err)
	}
	if rec.last.Method != http.MethodPost || rec.last.Path != mounts {
		t.Fatalf("request = %s %s", rec.last.Method, rec.last.Path)
	}
	want := map[string]any{
		"engine": "postgresql", "name": "main", "isolation": "schema_per_tenant",
		"connection_string": connStr,
	}
	if diff := cmpJSON(rec.last.Body, want); diff != "" {
		t.Fatalf("body: %s", diff)
	}
	if got, _ := decodeOut(t, h).(map[string]any); got["id"] != "m1" {
		t.Fatalf("output = %v", got)
	}
}

func TestDBAddConnectionStringFromEnv(t *testing.T) {
	rec := &recorder{body: `{"id":"m1","status":"created"}`}
	h := customHarness(t, rec, "json", func(k string) string {
		if k == "GROBASE_DB_CONNECTION_STRING" {
			return connStr
		}
		return ""
	}, "")
	if code := run(t, h, "add", "--name", "main", "--engine", "postgresql"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.Err)
	}
	if diff := cmpJSON(rec.last.Body, map[string]any{
		"engine": "postgresql", "name": "main", "connection_string": connStr,
	}); diff != "" {
		t.Fatalf("body: %s", diff)
	}
}

func TestDBAddCredentialRefInsteadOfConnectionString(t *testing.T) {
	rec := &recorder{body: `{"id":"m1","status":"created"}`}
	h := customHarness(t, rec, "json", func(k string) string {
		if k == "GROBASE_DB_CONNECTION_STRING" {
			return connStr
		}
		return ""
	}, "")
	args := []string{"add", "--name", "main", "--engine", "postgresql", "--credential-ref", "vault:prod/db"}
	if code := run(t, h, args...); code != 0 {
		t.Fatalf("exit %d: %s", code, h.Err)
	}
	if diff := cmpJSON(rec.last.Body, map[string]any{
		"engine": "postgresql", "name": "main", "credential_ref": "vault:prod/db",
	}); diff != "" {
		t.Fatalf("body: %s", diff)
	}
}

func TestDBAddWithoutAnyCredentialSource(t *testing.T) {
	rec := &recorder{body: `{"id":"m1","status":"created"}`}
	h := newHarness(t, rec, "")
	if code := run(t, h, "add", "--name", "main", "--engine", "postgresql"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.Err)
	}
	if diff := cmpJSON(rec.last.Body, map[string]any{"engine": "postgresql", "name": "main"}); diff != "" {
		t.Fatalf("body: %s", diff)
	}
}

func TestDBAddNeverEchoesTheConnectionString(t *testing.T) {
	rec := &recorder{body: `{"id":"m1","status":"created","connection_string":"` + connStr + `","dsn":"` + connStr + `"}`}
	h := newHarness(t, rec, connStr+"\n")
	args := []string{"add", "--name", "main", "--engine", "postgresql", "--connection-string-stdin"}
	if code := run(t, h, args...); code != 0 {
		t.Fatalf("exit %d: %s", code, h.Err)
	}
	if strings.Contains(h.Out.String()+h.Err.String(), connStr) {
		t.Fatalf("the connection string reached the output:\n%s\n%s", h.Out, h.Err)
	}
	if got, _ := decodeOut(t, h).(map[string]any); len(got) != 2 {
		t.Fatalf("output kept secret fields: %v", got)
	}
}

// cmpJSON compares two decoded values, reporting the first difference.
func cmpJSON(got, want any) string {
	g, _ := json.Marshal(got)
	w, _ := json.Marshal(want)
	if string(g) == string(w) {
		return ""
	}
	return "got " + string(g) + ", want " + string(w)
}
