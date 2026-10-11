package apicmd_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/apicmd"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const anonKey = "anon-key-api-123"

type seen struct {
	method, uri, body string
	hits              int
}

func call(t *testing.T, status int, reply string, args ...string) (int, *seen, string) {
	t.Helper()
	s := &seen{}
	h := testenv.New(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.method, s.uri, s.body, s.hits = r.Method, r.URL.RequestURI(), string(b), s.hits+1
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}), map[string]string{secret.KindAnon: anonKey}, "")
	code := exitcode.Of(h.Run(apicmd.New(h.Env), append(args, "--mode", "anon")...))
	return code, s, h.Out.String() + h.Err.String()
}

func TestAPIRequestShape(t *testing.T) {
	code, s, _ := call(t, 200, `{"ok":true}`, "post", "/rest/v1/things?x=1", "-f", "name=a", "-F", "n=5", "-F", "b=true")
	if code != 0 || s.method != "POST" || s.uri != "/rest/v1/things?x=1" {
		t.Fatalf("exit %d, sent %s %s", code, s.method, s.uri)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(s.body), &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "a" || got["n"] != float64(5) || got["b"] != true {
		t.Fatalf("body = %s", s.body)
	}
}

func TestAPIGetFieldsGoToTheQuery(t *testing.T) {
	_, s, _ := call(t, 200, `[]`, "GET", "/rest/v1/things", "-f", "limit=2")
	if s.method != "GET" || s.uri != "/rest/v1/things?limit=2" || s.body != "" {
		t.Fatalf("sent %s %s body=%q", s.method, s.uri, s.body)
	}
}

func TestAPIRejectsForeignPaths(t *testing.T) {
	for _, p := range []string{"rest/v1/x", "//evil.example/x", "/x?u=http://a", "https://evil.example/x"} {
		code, s, _ := call(t, 200, `{}`, "GET", p)
		if code != exitcode.Usage || s.hits != 0 {
			t.Errorf("%q: exit %d, hits %d", p, code, s.hits)
		}
	}
}

func TestAPIStatusToExitCode(t *testing.T) {
	for status, want := range map[int]int{401: exitcode.AuthFail, 403: exitcode.AuthFail, 404: exitcode.NotFound, 503: exitcode.Failure} {
		code, _, out := call(t, status, `{"msg":"x"}`, "GET", "/x")
		if code != want {
			t.Errorf("%d: exit %d, want %d", status, code, want)
		}
		if strings.Contains(out, anonKey) {
			t.Errorf("%d: key leaked", status)
		}
	}
}

func TestAPIBadFlags(t *testing.T) {
	for _, args := range [][]string{{"GET", "/x", "-f", "novalue"}, {"GET", "/x", "-H", "nocolon"}, {"GET", "/x", "--max-pages", "0"}, {"GET"}} {
		if code, s, _ := call(t, 200, `{}`, args...); code != exitcode.Usage || s.hits != 0 {
			t.Errorf("%v: exit %d, hits %d", args, code, s.hits)
		}
	}
}
