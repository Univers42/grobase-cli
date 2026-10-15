package functions_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/functions"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const serviceKey = "service-role-fn-123"

type seen struct {
	method, uri, ctype, auth, body, tenant string
	hits                                   int
}

func call(t *testing.T, status int, reply, stdin string, args ...string) (int, *seen, string) {
	t.Helper()
	s := &seen{}
	h := testenv.New(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.method, s.uri, s.ctype, s.auth, s.body = r.Method, r.URL.RequestURI(), r.Header.Get("Content-Type"), r.Header.Get("Authorization"), string(b)
		s.tenant = r.Header.Get("X-Baas-Tenant-Id")
		s.hits++
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}), map[string]string{secret.KindServiceRole: serviceKey, secret.KindAnon: "anon"}, stdin)
	err := h.Run(functions.New(h.Env), args...)
	return exitcode.Of(err), s, h.Out.String() + h.Err.String()
}

// The webhook-dispatcher scopes the triggers by tenant and answered 401 "missing tenant
// header" to a list that sent none.
func TestListSendsTheTenantWithTheServiceRole(t *testing.T) {
	code, s, out := call(t, 200, `[{"name":"f"}]`, "", "list", "--tenant", "agency")
	if code != 0 || s.uri != "/admin/v1/function-triggers" || s.auth != "Bearer "+serviceKey || !strings.Contains(out, `"f"`) {
		t.Fatalf("exit %d, %+v, %s", code, s, out)
	}
	if s.tenant != "agency" {
		t.Fatalf("X-Baas-Tenant-Id = %q, want agency", s.tenant)
	}
}

func TestListWithoutTenantIsAUsageErrorNamingTheFlag(t *testing.T) {
	hits := 0
	h := testenv.New(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }),
		map[string]string{secret.KindServiceRole: serviceKey}, "")
	err := h.Run(functions.New(h.Env), "list")
	if exitcode.Of(err) != exitcode.Usage || hits != 0 || !strings.Contains(fmt.Sprint(err), "--tenant") {
		t.Fatalf("exit %d, hits %d, err %v: want a usage error naming --tenant and no request", exitcode.Of(err), hits, err)
	}
}

func TestDeployStreamsTheFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "f.js")
	if err := os.WriteFile(src, []byte("export default 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, s, _ := call(t, 200, `{}`, "", "deploy", "my fn", "--file", src)
	if code != 0 || s.method != "PUT" || s.uri != "/functions/v1/my%20fn" || s.body != "export default 1" || s.ctype != "application/javascript" {
		t.Fatalf("exit %d, %+v", code, s)
	}
}

func TestInvoke(t *testing.T) {
	code, s, out := call(t, 200, `{"ok":true}`, "", "invoke", "f", "--data", `{"a":1}`)
	if code != 0 || s.method != "POST" || s.uri != "/functions/v1/f/invoke" || s.body != `{"a":1}` || !strings.Contains(out, "ok") {
		t.Fatalf("exit %d, %+v, %s", code, s, out)
	}
	_, _, out = call(t, 200, "plain text", "", "invoke", "f")
	if out != "plain text" {
		t.Fatalf("raw body = %q", out)
	}
	if code, _, _ = call(t, 200, `{}`, `{"b":2}`, "invoke", "f", "--data-stdin"); code != 0 {
		t.Fatalf("stdin: exit %d", code)
	}
}

func TestLogsQueryAndNotEnabled(t *testing.T) {
	code, s, _ := call(t, 200, `[]`, "", "logs", "f", "--tail", "7")
	if code != 0 || s.uri != "/functions/v1/f/logs?tail=7" {
		t.Fatalf("exit %d, %+v", code, s)
	}
	if _, _, out := call(t, 404, `{}`, "", "logs", "f"); out != "" {
		t.Fatalf("unexpected output %q", out)
	}
}

func TestStatusToExitCode(t *testing.T) {
	for _, args := range [][]string{{"list", "--tenant", "t1"}, {"invoke", "f"}, {"logs", "f"}} {
		for status, want := range map[int]int{401: exitcode.AuthFail, 403: exitcode.AuthFail, 404: exitcode.NotFound, 502: exitcode.Failure} {
			if code, _, out := call(t, status, `{"msg":"x"}`, "", args...); code != want || strings.Contains(out, serviceKey) {
				t.Errorf("%v %d: exit %d, want %d", args, status, code, want)
			}
		}
	}
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{
		{"deploy", "f"},
		{"deploy"},
		{"invoke"},
		{"invoke", "f", "--data", "{bad"},
		{"invoke", "f", "--data", "{}", "--data-stdin"},
		{"logs", "f", "--tail", "-1"},
		{"list", "x"},
	} {
		if code, s, _ := call(t, 200, `{}`, "", args...); code != exitcode.Usage || s.hits != 0 {
			t.Errorf("%v: exit %d hits %d", args, code, s.hits)
		}
	}
}
