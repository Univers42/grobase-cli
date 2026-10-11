package storage_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Univers42/grobase-cli/internal/cmd/storage"
	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const (
	apiKey      = "mbk_storage_test_key"
	bucketsPath = "/storage/v1/bucket"
)

// call is one request as it reached the server.
type call struct {
	Method, Path, Query string
	Body                map[string]any
	Auth, APIKey        string
	Upsert              string
	ContentType         string
	ContentLength       int64
}

// server answers every call with a fixed status and body. pre, when set, owns the
// response instead: streaming tests read the request body themselves, in chunks.
type server struct {
	status int
	body   string
	pre    func(w http.ResponseWriter, r *http.Request)

	mu    sync.Mutex
	calls []call
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.record(r)
	if s.pre != nil {
		s.pre(w, r)
		return
	}
	if s.status != 0 {
		w.WriteHeader(s.status)
	}
	_, _ = io.WriteString(w, s.body)
}

func (s *server) record(r *http.Request) {
	c := call{
		Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery,
		Auth: r.Header.Get("Authorization"), APIKey: r.Header.Get("apikey"),
		Upsert: r.Header.Get("x-upsert"), ContentType: r.Header.Get("Content-Type"),
		ContentLength: r.ContentLength,
	}
	if s.pre == nil {
		b, _ := io.ReadAll(r.Body)
		if len(strings.TrimSpace(string(b))) > 0 {
			_ = json.Unmarshal(b, &c.Body)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, c)
}

func (s *server) last(t *testing.T) call {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		t.Fatal("the server saw no request")
	}
	return s.calls[len(s.calls)-1]
}

func (s *server) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// run executes the `storage` command tree, keeping its error for the exit code.
func run(t *testing.T, srv *server, stdin string, args ...string) (*testenv.Harness, error) {
	t.Helper()
	h := testenv.New(t, srv, map[string]string{secret.KindAPIKey: apiKey}, stdin)
	return h, h.Run(storage.New(h.Env), args...)
}

// tree runs the tree and returns the exit code the CLI would use.
func tree(t *testing.T, srv *server, args ...string) int {
	t.Helper()
	_, err := run(t, srv, "", args...)
	return exitcode.Of(err)
}

// errorTable checks the exit codes every failing status must produce, and that
// only the collection root of a feature announces "not enabled on this
// deployment" (a 404 naming one object stays "not found").
func errorTable(t *testing.T, args []string, notEnabled bool) {
	t.Helper()
	for _, tc := range []struct {
		name   string
		status int
		want   int
	}{
		{"401", http.StatusUnauthorized, 3},
		{"403", http.StatusForbidden, 3},
		{"404", http.StatusNotFound, 4},
		{"500", http.StatusInternalServerError, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := &server{status: tc.status, body: `{"message":"boom"}`}
			if code := tree(t, srv, args...); code != tc.want {
				t.Fatalf("exit %d, want %d", code, tc.want)
			}
		})
	}
	if notEnabled {
		srv := &server{status: http.StatusNotFound, body: `{"message":"no such route"}`}
		_, err := run(t, srv, "", args...)
		if err == nil || !strings.Contains(err.Error(), "not enabled on this deployment:") {
			t.Fatalf("the root 404 was not relabelled (err=%v)", err)
		}
	}
}

// tableHarness runs the tree with a table output: testenv.New hard-codes json,
// and the list commands promise table columns.
func tableHarness(t *testing.T, srv *server) *testenv.Harness {
	t.Helper()
	s := httptest.NewServer(srv)
	t.Cleanup(s.Close)
	h := &testenv.Harness{
		Server: s, Store: secret.NewMemory(), Out: &bytes.Buffer{}, Err: &bytes.Buffer{},
		Stdin: strings.NewReader(""), Now: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	h.Env = env.New(env.Deps{
		Stdin: h.Stdin, Stdout: h.Out, Stderr: h.Err,
		Clock:        func() time.Time { return h.Now },
		Getenv:       func(string) string { return "" },
		ConfigPath:   t.TempDir() + "/config.json",
		Interactive:  func() bool { return false },
		ReadPassword: func(string) (string, error) { return "", nil },
		OpenSecrets:  func(secret.Options) (secret.Store, string, error) { return h.Store, "memory", nil },
		Version:      "test",
	}, &env.Flags{Output: "table", Timeout: 10 * time.Second})
	cf := &config.File{}
	if err := cf.Add(config.Context{Name: testenv.ContextName, URL: s.URL}); err != nil {
		t.Fatal(err)
	}
	if err := h.Env.ConfigStore().Save(cf); err != nil {
		t.Fatal(err)
	}
	if err := h.Store.Set(secret.Name(testenv.ContextName, secret.KindAPIKey), apiKey); err != nil {
		t.Fatal(err)
	}
	return h
}

func decodeOut(t *testing.T, h *testenv.Harness) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(h.Out.Bytes(), &v); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, h.Out)
	}
	return v
}
