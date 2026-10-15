package httpx_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Univers42/grobase-cli/internal/httpx"
	"github.com/Univers42/grobase-cli/internal/redact"
)

// modeRecorder is an Authorizer that only records the mode it was asked for.
type modeRecorder struct{ mode httpx.Mode }

func (m *modeRecorder) Authorize(_ context.Context, _ *http.Request, mode httpx.Mode) error {
	m.mode = mode
	return nil
}

func (*modeRecorder) Refresh(context.Context) (bool, error) { return false, nil }

func TestTenantRequestSendsTheTenantWithTheServiceRole(t *testing.T) {
	var tenant, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		tenant, body = r.Header.Get("X-Baas-Tenant-Id"), string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	base, _ := url.Parse(srv.URL)
	auth := &modeRecorder{}
	cl := &httpx.Client{Base: base, HTTP: srv.Client(), Auth: auth, Redact: redact.New()}
	req := httpx.TenantRequest(http.MethodPost, "/admin/v1/x", "agency", map[string]string{"k": "v"})
	if err := cl.JSON(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	if tenant != "agency" || body != `{"k":"v"}` || auth.mode != httpx.ModeService {
		t.Fatalf("tenant=%q body=%q mode=%v, want agency, the JSON body and the service role", tenant, body, auth.mode)
	}
}
