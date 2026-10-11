package query_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/query"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const (
	anonKey     = "anon-key-query-123"
	execPath    = "/query/v1/execute"
	enginesPath = "/query/v1/engines"
)

// sqlPath is a placeholder the tests replace with a real file holding select 1.
const sqlStatement = "select 1"

func run(t *testing.T, h http.Handler, stdin string, args ...string) (*testenv.Harness, error) {
	t.Helper()
	env := testenv.New(t, h, map[string]string{secret.KindAnon: anonKey}, stdin)
	return env, env.Run(query.New(env.Env), args...)
}

func statusHandler(status int) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(status) })
}

// captured is what a handler saw; empty want fields skip the check.
type captured struct {
	method, path, query string
	apiKey, auth        string
	body                []byte

	wantMethod, wantPath, wantQuery, wantBody string
	wantAuth                                  bool
}

func (c captured) check(t *testing.T) {
	t.Helper()
	if c.wantMethod != "" && c.method != c.wantMethod {
		t.Errorf("method = %q, want %q", c.method, c.wantMethod)
	}
	if c.wantPath != "" && c.path != c.wantPath {
		t.Errorf("path = %q, want %q", c.path, c.wantPath)
	}
	if c.query != c.wantQuery {
		t.Errorf("query = %q, want %q", c.query, c.wantQuery)
	}
	if c.wantBody != "" && string(c.body) != c.wantBody {
		t.Errorf("body = %s, want %s", c.body, c.wantBody)
	}
	if c.wantAuth && (c.apiKey != anonKey || c.auth != "Bearer "+anonKey) {
		t.Errorf("auth headers: apikey=%q authorization=%q", c.apiKey, c.auth)
	}
}

// recordingHandler echoes the recorded request and answers with body.
func recordingHandler(body string, got *captured) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// EscapedPath keeps %2F escapes visible, so a Seg'd id can be asserted.
		got.method, got.path, got.query = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		got.apiKey, got.auth = r.Header.Get("apikey"), r.Header.Get("Authorization")
		got.body, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(rw, body)
	})
}

// argsWith replaces @SQLFILE in args with a file containing sqlStatement.
func argsWith(t *testing.T, args ...string) []string {
	t.Helper()
	out := make([]string, len(args))
	copy(out, args)
	for i, a := range out {
		if a == "@SQLFILE" {
			path := filepath.Join(t.TempDir(), "statement.sql")
			if err := os.WriteFile(path, []byte(sqlStatement+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			out[i] = path
		}
	}
	return out
}

func TestQueryRunRequestBodyShape(t *testing.T) {
	want := `{"action":"raw","database_id":"db1","payload":{"sql":"select 1"},"resource":""}`
	for _, tc := range []struct {
		name, stdin string
		args        []string
	}{
		{"inline sql", "", []string{"run", "--db", "db1", "--sql", sqlStatement}},
		{"sql file", "", []string{"run", "--db", "db1", "--file", "@SQLFILE"}},
		{"sql on stdin", "select 1\n", []string{"run", "--db", "db1", "--file", "-"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c captured
			c.wantMethod, c.wantPath, c.wantBody, c.wantAuth = "POST", execPath, want, true
			_, err := run(t, recordingHandler(`{"rows":[{"one":1}]}`, &c), tc.stdin, argsWith(t, tc.args...)...)
			if err != nil {
				t.Fatalf("exit %d: %v", exitcode.Of(err), err)
			}
			c.check(t)
		})
	}
}

func TestQueryRunPrintsRows(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"rows wrapper", `{"rows":[{"id":1,"name":"first"},{"id":2,"name":"second"}]}`, "second"},
		{"data wrapper", `{"data":[{"id":7}]}`, "7"},
		{"bare array", `[{"id":9}]`, "9"},
		{"opaque object", `{"status":"ok"}`, "ok"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c captured
			c.wantMethod, c.wantPath, c.wantAuth = "POST", execPath, true
			env, err := run(t, recordingHandler(tc.body, &c), "", "run", "--db", "db1", "--sql", sqlStatement)
			if err != nil {
				t.Fatalf("exit %d: %v", exitcode.Of(err), err)
			}
			if !strings.Contains(env.Out.String(), tc.want) {
				t.Fatalf("output %s does not mention %q", env.Out, tc.want)
			}
		})
	}
}

func TestQueryRunAsTable(t *testing.T) {
	var c captured
	c.wantMethod, c.wantPath, c.wantAuth = "POST", execPath, true
	env := testenv.New(t, recordingHandler(`{"rows":[{"id":1,"name":"a"}]}`, &c), map[string]string{secret.KindAnon: anonKey}, "")
	env.Env.Flags.Output = "table"
	if err := env.Run(query.New(env.Env), "run", "--db", "db1", "--sql", sqlStatement); err != nil {
		t.Fatalf("exit %d: %v", exitcode.Of(err), err)
	}
	out := env.Out.String()
	if !strings.Contains(out, "ID") || !strings.Contains(out, "NAME") || !strings.Contains(out, "a") {
		t.Fatalf("table output missing: %s", out)
	}
}

func TestQueryRunUsage(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"no db", []string{"run", "--sql", sqlStatement}, 2},
		{"no statement", []string{"run", "--db", "db1"}, 2},
		{"missing file", []string{"run", "--db", "db1", "--file", filepath.Join(t.TempDir(), "nope.sql")}, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c captured
			_, err := run(t, recordingHandler(`{}`, &c), "", tc.args...)
			if got := exitcode.Of(err); got != tc.want {
				t.Fatalf("exit %d, want %d (err=%v)", got, tc.want, err)
			}
			if c.method != "" {
				t.Fatalf("a request was sent for %v", tc.args)
			}
		})
	}
}

func TestQueryRunModes(t *testing.T) {
	t.Run("none sends no credentials", func(t *testing.T) {
		var c captured
		_, err := run(t, recordingHandler(`{"rows":[]}`, &c), "", "run", "--db", "db1", "--sql", "select 1", "--mode", "none")
		if err != nil {
			t.Fatalf("exit %d: %v", exitcode.Of(err), err)
		}
		if c.apiKey != "" || c.auth != "" {
			t.Fatalf("credentials sent with --mode none: %q %q", c.apiKey, c.auth)
		}
	})

	t.Run("user without a session is refused", func(t *testing.T) {
		var c captured
		_, err := run(t, recordingHandler(`{"rows":[]}`, &c), "", "run", "--db", "db1", "--sql", "select 1", "--mode", "user")
		if got := exitcode.Of(err); got != 3 {
			t.Fatalf("exit %d, want 3 (err=%v)", got, err)
		}
		if c.method != "" {
			t.Fatal("a request was sent without a session")
		}
	})

	t.Run("unknown mode is a usage error", func(t *testing.T) {
		var c captured
		_, err := run(t, recordingHandler(`{"rows":[]}`, &c), "", "run", "--db", "db1", "--sql", "select 1", "--mode", "jwt")
		if got := exitcode.Of(err); got != 2 {
			t.Fatalf("exit %d, want 2 (err=%v)", got, err)
		}
		if c.method != "" {
			t.Fatal("a request was sent for a bad --mode")
		}
	})
}
