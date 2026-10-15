package query_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/query"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const (
	appKey  = "mbk_query_app_key_789"
	userJWT = "user-jwt-query-456"
)

// sent is the credential a query request carried.
type sent struct{ auth, dataKey string }

func credentialRecorder(got *sent) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		got.auth, got.dataKey = r.Header.Get("Authorization"), r.Header.Get("X-Baas-Api-Key")
		_, _ = io.WriteString(rw, `{"rows":[]}`)
	})
}

// subcommands are every query subcommand, each one a call to /query/v1.
var subcommands = map[string][]string{
	"engines": {"engines"},
	"schema":  {"schema", "--db", "db1"},
	"run":     {"run", "--db", "db1", "--sql", "select 1"},
	"crud":    {"crud", "--db", "db1", "--action", "select", "--resource", "people"},
}

func sendQuery(t *testing.T, creds map[string]string, args ...string) sent {
	t.Helper()
	var got sent
	h := testenv.New(t, credentialRecorder(&got), creds, "")
	if err := h.Run(query.New(h.Env), args...); err != nil {
		t.Fatalf("%v: exit %d: %v", args, exitcode.Of(err), err)
	}
	return got
}

// The query service accepts only an app key: with a session and an API key stored,
// auto mode sent the session and `grobase query engines` answered 401
// invalid_api_key (b2b, 2026-10-15).
func TestQueryAutoSendsTheStoredAPIKeyOverTheSession(t *testing.T) {
	creds := map[string]string{secret.KindAnon: anonKey, secret.KindSession: testenv.Session(t, userJWT), secret.KindAPIKey: appKey}
	for name, args := range subcommands {
		got := sendQuery(t, creds, args...)
		if got.dataKey != appKey || got.auth != "Bearer "+appKey {
			t.Errorf("%s: X-Baas-Api-Key=%q Authorization=%q, want the API key", name, got.dataKey, got.auth)
		}
	}
}

func TestQueryAutoWithOnlyASessionSendsTheSession(t *testing.T) {
	creds := map[string]string{secret.KindAnon: anonKey, secret.KindSession: testenv.Session(t, userJWT)}
	for name, args := range subcommands {
		got := sendQuery(t, creds, args...)
		if got.auth != "Bearer "+userJWT || got.dataKey != "" {
			t.Errorf("%s: Authorization=%q X-Baas-Api-Key=%q, want the session only", name, got.auth, got.dataKey)
		}
	}
}

func TestQueryExplicitModeOutranksTheAPIKey(t *testing.T) {
	creds := map[string]string{secret.KindAnon: anonKey, secret.KindSession: testenv.Session(t, userJWT), secret.KindAPIKey: appKey}
	got := sendQuery(t, creds, "engines", "--mode", "user")
	if got.auth != "Bearer "+userJWT || got.dataKey != "" {
		t.Fatalf("--mode user: Authorization=%q X-Baas-Api-Key=%q, want the session", got.auth, got.dataKey)
	}
}
