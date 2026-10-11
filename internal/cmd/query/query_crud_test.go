package query_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/exitcode"
)

func TestQueryCrudRequestBody(t *testing.T) {
	for _, tc := range []struct {
		name, stdin string
		args        []string
		want        string
	}{
		{
			"select with filter", "",
			[]string{"crud", "--db", "db1", "--action", "select", "--resource", "people", "--filter", `{"age":{"gt":30}}`},
			`{"action":"select","database_id":"db1","payload":{"filter":{"age":{"gt":30}}},"resource":"people"}`,
		},
		{
			"insert with data", "",
			[]string{"crud", "--db", "db1", "--action", "insert", "--resource", "people", "--data", `{"name":"ana"}`},
			`{"action":"insert","database_id":"db1","payload":{"data":{"name":"ana"}},"resource":"people"}`,
		},
		{
			"delete plain", "",
			[]string{"crud", "--db", "db1", "--action", "delete", "--resource", "people"},
			`{"action":"delete","database_id":"db1","payload":{},"resource":"people"}`,
		},
		{
			"data from stdin", `{"name":"zed"}`,
			[]string{"crud", "--db", "db1", "--action", "insert", "--resource", "people", "--data-stdin"},
			`{"action":"insert","database_id":"db1","payload":{"data":{"name":"zed"}},"resource":"people"}`,
		},
		{
			"data from file", "",
			[]string{"crud", "--db", "db1", "--action", "update", "--resource", "people", "--data-file", "@DATAFILE", "--filter", `{"id":3}`},
			`{"action":"update","database_id":"db1","payload":{"data":{"name":"bo"},"filter":{"id":3}},"resource":"people"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c captured
			c.wantMethod, c.wantPath, c.wantBody, c.wantAuth = "POST", execPath, tc.want, true
			args := argsWithDataFile(t, tc.args...)
			_, err := run(t, recordingHandler(`{"rows":[{"id":1}]}`, &c), tc.stdin, args...)
			if err != nil {
				t.Fatalf("exit %d: %v", exitcode.Of(err), err)
			}
			c.check(t)
		})
	}
}

// argsWithDataFile replaces @DATAFILE with a file holding one row.
func argsWithDataFile(t *testing.T, args ...string) []string {
	t.Helper()
	out := make([]string, len(args))
	copy(out, args)
	for i, a := range out {
		if a == "@DATAFILE" {
			path := filepath.Join(t.TempDir(), "row.json")
			if err := os.WriteFile(path, []byte(`{"name":"bo"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			out[i] = path
		}
	}
	return out
}

func TestQueryCrudUsage(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no db", []string{"crud", "--action", "select", "--resource", "t"}},
		{"bad action", []string{"crud", "--db", "db1", "--action", "truncate", "--resource", "t"}},
		{"no resource", []string{"crud", "--db", "db1", "--action", "select"}},
		{"insert without data", []string{"crud", "--db", "db1", "--action", "insert", "--resource", "t"}},
		{"update without data", []string{"crud", "--db", "db1", "--action", "update", "--resource", "t"}},
		{"bad filter json", []string{"crud", "--db", "db1", "--action", "select", "--resource", "t", "--filter", "nope"}},
		{"bad data json", []string{"crud", "--db", "db1", "--action", "insert", "--resource", "t", "--data", "{oops"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c captured
			_, err := run(t, recordingHandler(`{}`, &c), "", tc.args...)
			if got := exitcode.Of(err); got != 2 {
				t.Fatalf("exit %d, want 2 (err=%v)", got, err)
			}
			if c.method != "" {
				t.Fatalf("a request was sent for %v", tc.args)
			}
		})
	}
}

func TestQueryEngines(t *testing.T) {
	var c captured
	c.wantMethod, c.wantPath, c.wantAuth = "GET", enginesPath, true
	env, err := run(t, recordingHandler(`{"engines":[{"name":"postgres","version":16}]}`, &c), "", "engines")
	if err != nil {
		t.Fatalf("exit %d: %v", exitcode.Of(err), err)
	}
	c.check(t)
	if !strings.Contains(env.Out.String(), "postgres") {
		t.Fatalf("engines missing from output: %s", env.Out)
	}
}

func TestQuerySchemaEscapesId(t *testing.T) {
	var c captured
	c.wantMethod, c.wantPath, c.wantAuth = "GET", "/query/v1/db%2F1/schema", true
	_, err := run(t, recordingHandler(`{"tables":[{"name":"people"}]}`, &c), "", "schema", "--db", "db/1")
	if err != nil {
		t.Fatalf("exit %d: %v", exitcode.Of(err), err)
	}
	c.check(t)

	t.Run("404 on one id stays not found", func(t *testing.T) {
		_, err := run(t, statusHandler(http.StatusNotFound), "", "schema", "--db", "db1")
		if got := exitcode.Of(err); got != 4 {
			t.Fatalf("exit %d, want 4 (err=%v)", got, err)
		}
	})
}

func TestQueryFeatureOffCodes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		status     int
		want       int
		notEnabled bool
	}{
		{"run 401", []string{"run", "--db", "db1", "--sql", "select 1"}, 401, 3, false},
		{"run 403", []string{"run", "--db", "db1", "--sql", "select 1"}, 403, 3, false},
		{"run 404 off", []string{"run", "--db", "db1", "--sql", "select 1"}, 404, 4, true},
		{"run 500", []string{"run", "--db", "db1", "--sql", "select 1"}, 500, 1, false},
		{"engines 401", []string{"engines"}, 401, 3, false},
		{"engines 404 off", []string{"engines"}, 404, 4, true},
		{"engines 500", []string{"engines"}, 500, 1, false},
		{"crud 403", []string{"crud", "--db", "db1", "--action", "select", "--resource", "t"}, 403, 3, false},
		{"crud 404 off", []string{"crud", "--db", "db1", "--action", "select", "--resource", "t"}, 404, 4, true},
		{"crud 500", []string{"crud", "--db", "db1", "--action", "select", "--resource", "t"}, 500, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := run(t, statusHandler(tc.status), "", tc.args...)
			if got := exitcode.Of(err); got != tc.want {
				t.Fatalf("exit %d, want %d (err=%v)", got, tc.want, err)
			}
			has := err != nil && strings.Contains(err.Error(), "not enabled on this deployment")
			if has != tc.notEnabled {
				t.Fatalf("not-enabled=%v, want %v (err=%v)", has, tc.notEnabled, err)
			}
		})
	}
}

func TestQueryBadArgs(t *testing.T) {
	for _, args := range [][]string{{"run", "extra"}, {"engines", "x"}, {"schema"}} {
		var c captured
		if _, err := run(t, recordingHandler(`{}`, &c), "", args...); exitcode.Of(err) != 2 {
			t.Errorf("%v: exit %d, want 2", args, exitcode.Of(err))
		}
	}
}
