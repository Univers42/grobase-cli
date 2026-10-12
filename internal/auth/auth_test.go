package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Univers42/grobase-cli/internal/auth"
	"github.com/Univers42/grobase-cli/internal/httpx"
	"github.com/Univers42/grobase-cli/internal/redact"
	"github.com/Univers42/grobase-cli/internal/secret"
)

var now = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

type fixture struct {
	res      *auth.Resolver
	store    *secret.Memory
	refreshN atomic.Int32
}

func newFixture(t *testing.T, refresh http.HandlerFunc) *fixture {
	t.Helper()
	f := &fixture{store: secret.NewMemory()}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/token" || r.URL.Query().Get("grant_type") != "refresh_token" {
			http.NotFound(w, r)
			return
		}
		f.refreshN.Add(1)
		refresh(w, r)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	rd := redact.New()
	gt := &auth.GoTrue{Client: &httpx.Client{Base: u, HTTP: srv.Client(), Redact: rd}, Clock: func() time.Time { return now }}
	f.res = &auth.Resolver{Context: "c", Store: f.store, Clock: func() time.Time { return now }, Redact: rd, Gate: gt}
	return f
}

func (f *fixture) put(t *testing.T, kind, v string) {
	t.Helper()
	if err := f.store.Set(secret.Name("c", kind), v); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) putSession(t *testing.T, s auth.Session) {
	t.Helper()
	enc, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	f.put(t, secret.KindSession, enc)
}

func okRefresh(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`))
}

func headers(t *testing.T, f *fixture, mode httpx.Mode) (http.Header, error) {
	t.Helper()
	req, _ := http.NewRequest("GET", "http://x/", nil)
	err := f.res.Authorize(context.Background(), req, mode)
	return req.Header, err
}

func TestAuthorizeHeadersPerMode(t *testing.T) {
	f := newFixture(t, okRefresh)
	f.put(t, secret.KindAnon, "anon-key")
	f.put(t, secret.KindServiceRole, "service-key")
	f.put(t, secret.KindAPIKey, "mbk_abcdef123")
	f.putSession(t, auth.Session{AccessToken: "user-jwt", RefreshToken: "r", ExpiresAt: now.Add(time.Hour)})
	for _, tc := range []struct {
		mode           httpx.Mode
		apikey, bearer string
	}{
		{httpx.ModeUser, "anon-key", "Bearer user-jwt"},
		{httpx.ModeService, "service-key", "Bearer service-key"},
		{httpx.ModeAPIKey, "anon-key", "Bearer mbk_abcdef123"},
		{httpx.ModeAnon, "anon-key", "Bearer anon-key"},
		{httpx.ModeAuto, "anon-key", "Bearer user-jwt"},
	} {
		h, err := headers(t, f, tc.mode)
		if err != nil || h.Get("apikey") != tc.apikey || h.Get("Authorization") != tc.bearer {
			t.Errorf("mode %v: apikey=%q auth=%q err=%v", tc.mode, h.Get("apikey"), h.Get("Authorization"), err)
		}
		// The query service reads the app key only from X-Baas-Api-Key; no other
		// credential may ever be sent under that name.
		wantDataPlane := ""
		if tc.mode == httpx.ModeAPIKey {
			wantDataPlane = "mbk_abcdef123"
		}
		if got := h.Get("X-Baas-Api-Key"); got != wantDataPlane {
			t.Errorf("mode %v: X-Baas-Api-Key=%q, want %q", tc.mode, got, wantDataPlane)
		}
	}
}

func TestAutoFallsBackWithoutSession(t *testing.T) {
	f := newFixture(t, okRefresh)
	f.put(t, secret.KindAnon, "anon-key")
	h, err := headers(t, f, httpx.ModeAuto)
	if err != nil || h.Get("Authorization") != "Bearer anon-key" {
		t.Fatalf("anon fallback: %v %v", h, err)
	}
	f.put(t, secret.KindServiceRole, "service-key")
	if h, _ = headers(t, f, httpx.ModeAuto); h.Get("Authorization") != "Bearer service-key" {
		t.Fatalf("service fallback: %v", h)
	}
}

func TestMissingCredentialIsErrNoCredential(t *testing.T) {
	f := newFixture(t, okRefresh)
	for _, m := range []httpx.Mode{httpx.ModeService, httpx.ModeAnon, httpx.ModeAPIKey, httpx.ModeUser} {
		if _, err := headers(t, f, m); !errors.Is(err, auth.ErrNoCredential) {
			t.Errorf("mode %v: err=%v, want ErrNoCredential", m, err)
		}
	}
}

func TestProactiveRefreshPersistsNewSession(t *testing.T) {
	f := newFixture(t, okRefresh)
	f.put(t, secret.KindAnon, "anon-key")
	f.putSession(t, auth.Session{AccessToken: "old", RefreshToken: "old-r", ExpiresAt: now.Add(30 * time.Second)})
	h, err := headers(t, f, httpx.ModeUser)
	if err != nil || h.Get("Authorization") != "Bearer new-access" || f.refreshN.Load() != 1 {
		t.Fatalf("auth=%q refreshes=%d err=%v", h.Get("Authorization"), f.refreshN.Load(), err)
	}
	stored, _ := f.store.Get(secret.Name("c", secret.KindSession))
	if !strings.Contains(stored, "new-refresh") {
		t.Fatalf("refreshed session not persisted: %s", stored)
	}
	// A fresh token is not refreshed again.
	if _, err := headers(t, f, httpx.ModeUser); err != nil || f.refreshN.Load() != 1 {
		t.Fatalf("second call refreshed again (n=%d, err=%v)", f.refreshN.Load(), err)
	}
}

func TestRefreshFailure(t *testing.T) {
	deny := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}
	t.Run("still valid token is used despite failed refresh", func(t *testing.T) {
		f := newFixture(t, deny)
		f.putSession(t, auth.Session{AccessToken: "old", RefreshToken: "r", ExpiresAt: now.Add(30 * time.Second)})
		h, err := headers(t, f, httpx.ModeUser)
		if err != nil || h.Get("Authorization") != "Bearer old" {
			t.Fatalf("got %q, %v", h.Get("Authorization"), err)
		}
	})
	t.Run("expired token plus failed refresh is an error", func(t *testing.T) {
		f := newFixture(t, deny)
		f.putSession(t, auth.Session{AccessToken: "old", RefreshToken: "r", ExpiresAt: now.Add(-time.Minute)})
		if _, err := headers(t, f, httpx.ModeUser); err == nil || !strings.Contains(err.Error(), "grobase login") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("forced refresh reports failure", func(t *testing.T) {
		f := newFixture(t, deny)
		f.putSession(t, auth.Session{AccessToken: "old", RefreshToken: "r", ExpiresAt: now.Add(time.Hour)})
		if ok, err := f.res.Refresh(context.Background()); ok || err == nil {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("no session means nothing to refresh", func(t *testing.T) {
		f := newFixture(t, deny)
		if ok, err := f.res.Refresh(context.Background()); ok || err != nil {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
}

func TestSessionExpiring(t *testing.T) {
	s := auth.Session{ExpiresAt: now.Add(time.Minute)}
	if s.Expiring(now, 30*time.Second) || !s.Expiring(now, 2*time.Minute) {
		t.Fatal("skew arithmetic wrong")
	}
	if (auth.Session{}).Expiring(now, time.Hour) {
		t.Fatal("zero expiry means never expiring")
	}
}

func TestSessionRoundTrip(t *testing.T) {
	in := auth.Session{AccessToken: "a", RefreshToken: "r", ExpiresAt: now, Email: "e@x"}
	enc, err := in.Encode()
	if err != nil {
		t.Fatal(err)
	}
	out, err := auth.DecodeSession(enc)
	if err != nil || out != in {
		t.Fatalf("%+v %v", out, err)
	}
	if _, err := auth.DecodeSession("{"); err == nil {
		t.Fatal("garbage must not decode")
	}
}
