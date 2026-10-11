package env

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Univers42/grobase-cli/internal/config"
)

// newProbeEnv builds an Env the way testenv.New does: injected I/O, a fixed
// clock, a temp config path and a stubbed getenv for probe timeouts.
func newProbeEnv(t *testing.T, getenv func(string) string) *Env {
	t.Helper()
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	return New(Deps{
		Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		Clock:       func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
		Getenv:      getenv,
		ConfigPath:  t.TempDir() + "/config.json",
		Interactive: func() bool { return false },
		Version:     "test",
	}, &Flags{})
}

// healthHandler serves 200 on every path and counts /auth/v1/health hits,
// sleeping that long on health probes when sleep is positive.
func healthHandler(calls *atomic.Int32, sleep time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/v1/health" {
			calls.Add(1)
			if sleep > 0 {
				time.Sleep(sleep)
			}
		}
		w.WriteHeader(http.StatusOK)
	})
}

// closedURL starts a server and closes it, returning a URL that refuses connections.
func closedURL(t *testing.T) string {
	t.Helper()
	s := httptest.NewServer(http.NotFoundHandler())
	u := s.URL
	s.Close()
	return u
}

func TestSelectedURLWithoutAlternatesSkipsProbe(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(healthHandler(&calls, 0))
	defer srv.Close()
	e := newProbeEnv(t, nil)
	got, err := e.SelectedURL(config.Context{Name: "dc", URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if got != srv.URL {
		t.Fatalf("got %q want %q", got, srv.URL)
	}
	if calls.Load() != 0 {
		t.Fatalf("health hits = %d want 0", calls.Load())
	}
}

func TestSelectedURLFallbackAndMemoization(t *testing.T) {
	var calls atomic.Int32
	up := httptest.NewServer(healthHandler(&calls, 0))
	defer up.Close()
	e := newProbeEnv(t, nil)
	c := config.Context{Name: "dc", URL: closedURL(t), Alternates: []string{up.URL}}
	got, err := e.SelectedURL(c)
	if err != nil {
		t.Fatal(err)
	}
	if got != up.URL {
		t.Fatalf("got %q want %q", got, up.URL)
	}
	if calls.Load() != 1 {
		t.Fatalf("health hits = %d want 1", calls.Load())
	}
	got2, err := e.SelectedURL(c)
	if err != nil {
		t.Fatal(err)
	}
	if got2 != up.URL || calls.Load() != 1 {
		t.Fatalf("memoized got %q hits %d want %q hits 1", got2, calls.Load(), up.URL)
	}
}

func TestSelectedURLAllClosedErrors(t *testing.T) {
	u1 := closedURL(t)
	u2 := closedURL(t)
	e := newProbeEnv(t, nil)
	_, err := e.SelectedURL(config.Context{Name: "dc", URL: u1, Alternates: []string{u2}})
	if err == nil || !strings.Contains(err.Error(), "no URL answered") {
		t.Fatalf("err = %v want no URL answered", err)
	}
	if !strings.Contains(err.Error(), u1) || !strings.Contains(err.Error(), u2) {
		t.Fatalf("err = %v want both URLs", err)
	}
}

func TestSelectedURLHonoursProbeTimeout(t *testing.T) {
	var slowCalls, fastCalls atomic.Int32
	slow := httptest.NewServer(healthHandler(&slowCalls, time.Second))
	defer slow.Close()
	fast := httptest.NewServer(healthHandler(&fastCalls, 0))
	defer fast.Close()
	getenv := func(k string) string {
		if k == "GROBASE_PROBE_TIMEOUT" {
			return "50ms"
		}
		return ""
	}
	e := newProbeEnv(t, getenv)
	c := config.Context{Name: "dc", URL: slow.URL, Alternates: []string{fast.URL}}
	start := time.Now()
	got, err := e.SelectedURL(c)
	if elapsed := time.Since(start); err != nil {
		t.Fatal(err)
	} else if got != fast.URL {
		t.Fatalf("got %q want %q", got, fast.URL)
	} else if elapsed >= 900*time.Millisecond {
		t.Fatalf("elapsed %s want < 900ms", elapsed)
	}
	if fastCalls.Load() != 1 {
		t.Fatalf("fast hits = %d want 1", fastCalls.Load())
	}
}
