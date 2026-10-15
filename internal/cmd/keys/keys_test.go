package keys_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/keys"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const (
	anonKey = "anon-key-keys-123"
	newKey  = "mbk_CLEARTEXT_v1_ROTATE_999"
	keysURL = "/v1/tenants/me/keys"
)

func run(t *testing.T, h http.Handler, args ...string) (*testenv.Harness, error) {
	t.Helper()
	env := testenv.New(t, h, map[string]string{secret.KindAnon: anonKey}, "")
	return env, env.Run(keys.New(env.Env), args...)
}

func statusHandler(status int) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(status)
	})
}

func bodyJSON(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	b, _ := io.ReadAll(r.Body)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode body %q: %v", b, err)
	}
	return m
}

func TestKeysListBothShapes(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"array", `[{"id":"k1","name":"first","scopes":["read"]},{"id":"k2","name":"second"}]`, 2},
		{"object", `{"keys":[{"id":"k1","name":"first"}]}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			env, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				if r.Header.Get("apikey") != anonKey {
					t.Errorf("apikey = %q", r.Header.Get("apikey"))
				}
				_, _ = io.WriteString(rw, tc.body)
			}), "list")
			if err != nil {
				t.Fatalf("exit %d: %v", exitcode.Of(err), err)
			}
			if gotMethod != "GET" || gotPath != keysURL {
				t.Fatalf("got %s %s", gotMethod, gotPath)
			}
			var got []map[string]any
			if e := json.Unmarshal(env.Out.Bytes(), &got); e != nil {
				t.Fatalf("output not a JSON array: %v\n%s", e, env.Out)
			}
			if len(got) != tc.want {
				t.Fatalf("got %d keys, want %d", len(got), tc.want)
			}
		})
	}
}

// The control plane keeps preferring the session: only /query/v1 ranks a stored API key first.
func TestKeysListSendsTheSessionOverAStoredAPIKey(t *testing.T) {
	var gotAuth, gotDataKey string
	h := testenv.New(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		gotAuth, gotDataKey = r.Header.Get("Authorization"), r.Header.Get("X-Baas-Api-Key")
		_, _ = io.WriteString(rw, `[]`)
	}), map[string]string{
		secret.KindAnon: anonKey, secret.KindSession: testenv.Session(t, "user-jwt-keys"), secret.KindAPIKey: "mbk_keys_app_key",
	}, "")
	if err := h.Run(keys.New(h.Env), "list"); err != nil {
		t.Fatalf("exit %d: %v", exitcode.Of(err), err)
	}
	if gotAuth != "Bearer user-jwt-keys" || gotDataKey != "" {
		t.Fatalf("Authorization=%q X-Baas-Api-Key=%q, want the session", gotAuth, gotDataKey)
	}
}

func TestKeysListErrorCodes(t *testing.T) {
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
			_, err := run(t, statusHandler(tc.status), "list")
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

func TestKeysCreate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		wantBody string
	}{
		{"name only", []string{"create", "--name", "ci"}, `{"name":"ci"}`},
		{"with scopes", []string{"create", "--name", "ci", "--scope", "read", "--scope", "write"}, `{"name":"ci","scopes":["read","write"]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			var body map[string]any
			env, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath, body = r.Method, r.URL.Path, bodyJSON(t, r)
				_, _ = io.WriteString(rw, `{"id":"k9","name":"ci","key":"`+newKey+`"}`)
			}), tc.args...)
			if err != nil {
				t.Fatalf("exit %d: %v", exitcode.Of(err), err)
			}
			assertCreate(t, gotMethod, gotPath, body, tc.wantBody, env)
		})
	}
}

func assertCreate(t *testing.T, method, path string, body map[string]any, wantBody string, env *testenv.Harness) {
	t.Helper()
	if method != "POST" || path != keysURL {
		t.Fatalf("got %s %s", method, path)
	}
	got, _ := json.Marshal(body)
	if string(got) != wantBody {
		t.Fatalf("body = %s, want %s", got, wantBody)
	}
	if !strings.Contains(env.Out.String(), newKey) {
		t.Fatalf("cleartext key missing from output: %s", env.Out)
	}
	if !strings.Contains(env.Err.String(), "store this key now") {
		t.Fatalf("warning missing: %s", env.Err)
	}
}

func TestKeysCreateRequiresName(t *testing.T) {
	called := false
	_, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		called = true
	}), "create")
	if got := exitcode.Of(err); got != 2 {
		t.Fatalf("exit %d, want 2 (err=%v)", got, err)
	}
	if called {
		t.Fatal("no request should be sent when --name is missing")
	}
}

func TestKeysRevoke(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var gotMethod, gotPath, gotAuth string
		env, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
			rw.WriteHeader(http.StatusNoContent)
		}), "revoke", "key-123", "--yes")
		if err != nil {
			t.Fatalf("exit %d: %v", exitcode.Of(err), err)
		}
		if gotMethod != http.MethodDelete || gotPath != keysURL+"/key-123" {
			t.Fatalf("got %s %s", gotMethod, gotPath)
		}
		if gotAuth != "Bearer "+anonKey {
			t.Errorf("Authorization = %q", gotAuth)
		}
		if !strings.Contains(env.Out.String(), "key-123") {
			t.Fatalf("revoked id missing: %s", env.Out)
		}
	})

	t.Run("refused without --yes", func(t *testing.T) {
		called := false
		_, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			called = true
		}), "revoke", "key-123")
		if got := exitcode.Of(err); got != 2 {
			t.Fatalf("exit %d, want 2", got)
		}
		if called {
			t.Fatal("DELETE was sent without confirmation")
		}
	})

	t.Run("missing id", func(t *testing.T) {
		_, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {}), "revoke")
		if got := exitcode.Of(err); got != 2 {
			t.Fatalf("exit %d, want 2", got)
		}
	})
}

func TestKeysRevokeErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		want       int
		notEnabled bool
	}{
		{"401", 401, 3, false},
		{"403", 403, 3, false},
		{"404 not found", 404, 4, false}, // a specific id stays "not found", never "not enabled"
		{"500", 500, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := run(t, statusHandler(tc.status), "revoke", "key-123", "--yes")
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
