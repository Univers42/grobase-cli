package db_test

import (
	"testing"

	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

// `db test` reads through /query/v1, which accepts only an app key, so a stored API
// key outranks the session there as it does for `grobase query`.
func TestDBTestSendsTheStoredAPIKeyOverTheSession(t *testing.T) {
	rec := &recorder{body: `{"tables":[]}`}
	h := testenv.New(t, rec, map[string]string{
		secret.KindSession: testenv.Session(t, "user-jwt-db"), secret.KindAPIKey: apiKey,
	}, "")
	if code := run(t, h, "test", "m1"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.Err)
	}
	if rec.last.DataKey != apiKey || rec.last.Auth != "Bearer "+apiKey {
		t.Fatalf("X-Baas-Api-Key=%q Authorization=%q, want the API key", rec.last.DataKey, rec.last.Auth)
	}
}
