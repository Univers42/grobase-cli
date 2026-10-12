package httpx_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Univers42/grobase-cli/internal/httpx"
	"github.com/Univers42/grobase-cli/internal/redact"
)

func pinOf(srv *httptest.Server) string {
	sum := sha256.Sum256(srv.Certificate().Raw)
	return hex.EncodeToString(sum[:])
}

func clientFor(t *testing.T, srv *httptest.Server, cfg httpx.TLS) *httpx.Client {
	t.Helper()
	tr, err := httpx.NewTransport(cfg, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(srv.URL)
	return &httpx.Client{Base: u, HTTP: &http.Client{Transport: tr}, Redact: redact.New()}
}

func get(c *httpx.Client) error {
	return c.JSON(context.Background(), httpx.Request{Method: "GET", Path: "/x", Mode: httpx.ModeNone}, nil)
}

func TestTLSTrustModes(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer srv.Close()
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(caFile, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	bad := strings.Repeat("ab", 32)
	cases := []struct {
		name    string
		cfg     httpx.TLS
		wantErr string
	}{
		{"no trust config rejects self-signed", httpx.TLS{}, "certificate"},
		{"right pin", httpx.TLS{PinSHA256: pinOf(srv)}, ""},
		{"wrong pin", httpx.TLS{PinSHA256: bad}, "pin mismatch"},
		{"insecure", httpx.TLS{Insecure: true}, ""},
		{"ca file", httpx.TLS{CAFile: caFile}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := get(clientFor(t, srv, tc.cfg))
			if tc.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestTLSConfigValidation(t *testing.T) {
	for name, cfg := range map[string]httpx.TLS{
		"short pin":      {PinSHA256: "abcd"},
		"non-hex pin":    {PinSHA256: strings.Repeat("zz", 32)},
		"missing CA":     {CAFile: filepath.Join(t.TempDir(), "nope.pem")},
		"CA without PEM": {CAFile: writeTemp(t, "not a cert")},
	} {
		if _, err := cfg.Config(); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if _, err := (httpx.TLS{Insecure: true}).Config(); err != nil {
		t.Fatal(err)
	}
}

func TestProbeReportsPin(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	info, err := httpx.Probe(context.Background(), u.Host)
	if err != nil {
		t.Fatal(err)
	}
	if info.SHA256 != pinOf(srv) {
		t.Fatalf("probe sha %s, want %s", info.SHA256, pinOf(srv))
	}
	if info.Verified {
		t.Fatal("self-signed test cert must not report Verified")
	}
}

func TestPinIsEnforcedInVerifyConnection(t *testing.T) {
	cfg, err := httpx.TLS{PinSHA256: strings.Repeat("00", 32)}.Config()
	if err != nil {
		t.Fatal(err)
	}
	// VerifyPeerCertificate is skipped on resumed sessions; VerifyConnection is not.
	if cfg.VerifyConnection == nil || cfg.VerifyPeerCertificate != nil {
		t.Fatal("pin must be enforced in VerifyConnection")
	}
}

type stubAuth struct {
	refreshed atomic.Int32
	ok        bool
}

func (s *stubAuth) Authorize(_ context.Context, r *http.Request, _ httpx.Mode) error {
	r.Header.Set("Authorization", fmt.Sprintf("Bearer tok%d", s.refreshed.Load()))
	return nil
}

func (s *stubAuth) Refresh(context.Context) (bool, error) { s.refreshed.Add(1); return s.ok, nil }

func TestRefreshRetryOnce(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") == "Bearer tok0" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	auth := &stubAuth{ok: true}
	c := &httpx.Client{Base: u, HTTP: srv.Client(), Auth: auth, Redact: redact.New()}
	var out map[string]any
	if err := c.JSON(context.Background(), httpx.Request{Method: "POST", Path: "/x", JSON: map[string]int{"a": 1}}, &out); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || auth.refreshed.Load() != 1 || out["ok"] != true {
		t.Fatalf("calls=%d refreshes=%d out=%v", calls.Load(), auth.refreshed.Load(), out)
	}
	// A non-replayable stream body is never retried.
	calls.Store(0)
	auth2 := &stubAuth{ok: true}
	c.Auth = auth2
	err := c.JSON(context.Background(), httpx.Request{Method: "PUT", Path: "/x", Body: strings.NewReader("data")}, nil)
	var api *httpx.APIError
	if !errors.As(err, &api) || api.Status != 401 || calls.Load() != 1 || auth2.refreshed.Load() != 0 {
		t.Fatalf("stream body must not be retried: err=%v calls=%d", err, calls.Load())
	}
}

func TestAPIErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		status       int
		body         string
		msg          string
		notFound, un bool
	}{
		{401, `{"msg":"bad jwt"}`, "bad jwt", false, true},
		{403, `{"message":"forbidden"}`, "forbidden", false, true},
		{404, `{"error":"nope"}`, "nope", true, false},
		{503, `upstream down`, "upstream down", false, false},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		u, _ := url.Parse(srv.URL)
		c := &httpx.Client{Base: u, HTTP: srv.Client(), Redact: redact.New()}
		err := get(c)
		srv.Close()
		var api *httpx.APIError
		if !errors.As(err, &api) {
			t.Fatalf("%d: not an APIError: %v", tc.status, err)
		}
		if api.Status != tc.status || api.NotFound() != tc.notFound || api.Unauthorized() != tc.un || !strings.Contains(err.Error(), tc.msg) {
			t.Errorf("%d: got %+v / %v", tc.status, api, err)
		}
	}
}

// The bodies are the ones the baas VM answered on 2026-10-12: Kong for /nonexistent-zz/v1/x,
// tenant-control (TENANT_SELFSERVE_ENABLED off) for /v1/tenants/me/mounts.
func TestMissTellsWhoAnswered404(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       httpx.Miss
		state      string
		prefix     string
	}{
		{
			"kong no route", `{"message":"no Route matched with those values","request_id":"0f3c"}`,
			httpx.MissNoRoute, "not deployed", "mounts: not deployed on this gateway: ",
		},
		{"go mux, feature off", "404 page not found\n", httpx.MissNoHandler, "switched off", "mounts: switched off on this deployment: "},
		{
			"express", `<!DOCTYPE html><html><body><pre>Cannot GET /x</pre></body></html>`,
			httpx.MissNoHandler, "switched off", "mounts: switched off on this deployment: ",
		},
		{
			"nestjs", `{"message":"Cannot GET /x","error":"Not Found","statusCode":404}`,
			httpx.MissNoHandler, "switched off", "mounts: switched off on this deployment: ",
		},
		{"a handler's json", `{"msg":"User not found"}`, httpx.MissUnknown, "not enabled", "mounts: not enabled on this deployment: "},
		{"empty", ``, httpx.MissUnknown, "not enabled", "mounts: not enabled on this deployment: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			u, _ := url.Parse(srv.URL)
			err := get(&httpx.Client{Base: u, HTTP: srv.Client(), Redact: redact.New()})
			var api *httpx.APIError
			if !errors.As(err, &api) {
				t.Fatalf("not an APIError: %v", err)
			}
			if api.Miss() != tc.want || api.Miss().FeatureState() != tc.state {
				t.Errorf("Miss = %d (%s), want %d (%s)", api.Miss(), api.Miss().FeatureState(), tc.want, tc.state)
			}
			if got := httpx.NotEnabled("mounts", err).Error(); !strings.HasPrefix(got, tc.prefix) {
				t.Errorf("NotEnabled = %q, want prefix %q", got, tc.prefix)
			}
		})
	}
	if (&httpx.APIError{Status: 500, Message: "no Route matched with those values"}).Miss() != httpx.MissUnknown {
		t.Error("a non-404 must never be classified")
	}
}

func TestDebugTraceRedactsCredentials(t *testing.T) {
	const jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.c2lnbmF0dXJl"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"` + jwt + `","key":"mbk_supersecretvalue1"}`))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	var dbg bytes.Buffer
	c := &httpx.Client{Base: u, HTTP: srv.Client(), Auth: headerAuth{jwt}, Debug: &dbg, Redact: redact.New("hunter2-password")}
	err := c.JSON(context.Background(), httpx.Request{
		Method: "POST", Path: "/x", JSON: map[string]string{"password": "hunter2-password"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	trace := dbg.String()
	for _, leak := range []string{jwt, "mbk_supersecretvalue1", "hunter2-password", "c2lnbmF0dXJl"} {
		if strings.Contains(trace, leak) {
			t.Errorf("debug trace leaks %q:\n%s", leak, trace)
		}
	}
	if !strings.Contains(trace, "POST") || !strings.Contains(trace, "[REDACTED]") {
		t.Errorf("trace missing request line or mask:\n%s", trace)
	}
}

type headerAuth struct{ tok string }

func (h headerAuth) Authorize(_ context.Context, r *http.Request, _ httpx.Mode) error {
	r.Header.Set("Authorization", "Bearer "+h.tok)
	r.Header.Set("apikey", h.tok)
	return nil
}
func (headerAuth) Refresh(context.Context) (bool, error) { return false, nil }

func writeTemp(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
