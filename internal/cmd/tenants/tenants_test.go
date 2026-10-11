package tenants_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/tenants"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const anonKey = "anon-key-tenants-123"

func run(t *testing.T, h http.Handler, args ...string) (*testenv.Harness, error) {
	t.Helper()
	env := testenv.New(t, h, map[string]string{secret.KindAnon: anonKey}, "")
	return env, env.Run(tenants.New(env.Env), args...)
}

func statusHandler(status int) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(status)
	})
}

func TestTenantsRoutesAndRequestShape(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		wantPath string
	}{
		{[]string{"show"}, "/v1/tenants/me"},
		{[]string{"usage"}, "/v1/tenants/me/usage"},
		{[]string{"entitlements"}, "/v1/tenants/me/entitlements"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var gotMethod, gotPath, gotAPIKey, gotAuth string
			env, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				gotAPIKey = r.Header.Get("apikey")
				gotAuth = r.Header.Get("Authorization")
				_, _ = io.WriteString(rw, `{"id":"t1","name":"acme"}`)
			}), tc.args...)
			if err != nil {
				t.Fatalf("exit %d: %v", exitcode.Of(err), err)
			}
			if gotMethod != "GET" {
				t.Errorf("method = %q, want GET", gotMethod)
			}
			if gotPath != tc.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tc.wantPath)
			}
			// ModeAuto with only an anon key stored sends it as apikey and bearer.
			if gotAPIKey != anonKey || gotAuth != "Bearer "+anonKey {
				t.Errorf("auth headers: apikey=%q authorization=%q", gotAPIKey, gotAuth)
			}
			var got map[string]any
			if e := json.Unmarshal(env.Out.Bytes(), &got); e != nil {
				t.Fatalf("output not JSON: %v\n%s", e, env.Out)
			}
			if got["name"] != "acme" {
				t.Errorf("name = %v", got["name"])
			}
		})
	}
}

func TestTenantsErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		want       int
		notEnabled bool
	}{
		{"401", 401, 3, false},
		{"403", 403, 3, false},
		{"404 not enabled", 404, 4, true},
		{"500", 500, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := run(t, statusHandler(tc.status), "show")
			if got := exitcode.Of(err); got != tc.want {
				t.Fatalf("exit %d, want %d (err=%v)", got, tc.want, err)
			}
			has := err != nil && strings.Contains(err.Error(), "not enabled on this deployment:")
			if has != tc.notEnabled {
				t.Fatalf("not-enabled=%v, want %v (err=%v)", has, tc.notEnabled, err)
			}
		})
	}
}

func TestTenantsBadUsage(t *testing.T) {
	for _, args := range [][]string{{"show", "extra"}, {"usage", "x", "y"}, {"entitlements", "z"}} {
		_, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			t.Fatal("no request should be sent on a usage error")
		}), args...)
		if got := exitcode.Of(err); got != 2 {
			t.Errorf("%v: exit %d, want 2 (err=%v)", args, got, err)
		}
	}
}
