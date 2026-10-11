package orgs_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/orgs"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

func TestOrgsRoutesAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		stdin  string
		status int
		method string
		path   string
		want   int
	}{
		{"list ok", []string{"list"}, "", 200, "GET", "/v1/orgs", 0},
		{"show ok", []string{"show", "o1"}, "", 200, "GET", "/v1/orgs/o1", 0},
		{"members list", []string{"members", "list", "o1"}, "", 200, "GET", "/v1/orgs/o1/members", 0},
		{"members remove", []string{"members", "remove", "o1", "u1", "--yes"}, "", 200, "DELETE", "/v1/orgs/o1/members/u1", 0},
		{"invites create", []string{"invites", "create", "o1", "--email", "a@b.co"}, "", 200, "POST", "/v1/orgs/o1/invites", 0},
		{"projects", []string{"projects", "o1"}, "", 200, "GET", "/v1/orgs/o1/projects", 0},
		{"usage", []string{"usage", "o1"}, "", 200, "GET", "/v1/orgs/o1/usage", 0},
		{"id is escaped", []string{"show", "a/b"}, "", 200, "GET", "/v1/orgs/a%2Fb", 0},
		{"401", []string{"list"}, "", 401, "GET", "/v1/orgs", exitcode.AuthFail},
		{"403", []string{"list"}, "", 403, "GET", "/v1/orgs", exitcode.AuthFail},
		{"404 on root means not enabled", []string{"list"}, "", 404, "GET", "/v1/orgs", exitcode.NotFound},
		{"404 on an org", []string{"show", "nope"}, "", 404, "GET", "/v1/orgs/nope", exitcode.NotFound},
		{"5xx", []string{"list"}, "", 503, "GET", "/v1/orgs", 1},
		{"show needs an id", []string{"show"}, "", 200, "", "", exitcode.Usage},
		{"remove declined", []string{"members", "remove", "o1", "u1"}, "n\n", 200, "", "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotURI string
			h := testenv.New(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotURI = r.Method, r.URL.EscapedPath()
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"msg":"x"}`))
			}), map[string]string{secret.KindAPIKey: "mbk_x"}, tc.stdin)
			got := exitcode.Of(h.Run(orgs.New(h.Env), tc.args...))
			if got != tc.want {
				t.Fatalf("exit %d, want %d: %s", got, tc.want, h.Err)
			}
			if tc.method != "" && (gotMethod != tc.method || gotURI != tc.path) {
				t.Fatalf("sent %s %s, want %s %s", gotMethod, gotURI, tc.method, tc.path)
			}
			if tc.method == "" && gotMethod != "" {
				t.Fatalf("a rejected command reached the server: %s %s", gotMethod, gotURI)
			}
			if strings.Contains(h.Out.String()+h.Err.String(), "mbk_x") {
				t.Fatal("api key leaked")
			}
		})
	}
}
