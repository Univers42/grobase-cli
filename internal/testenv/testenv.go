// Package testenv builds a hermetic *env.Env for command tests: an httptest server,
// an in-memory credential store, a temp config dir and captured stdout/stderr.
package testenv

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/auth"
	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/secret"
)

// ContextName is the name of the context the harness creates and selects.
const ContextName = "test"

// Harness bundles the fake world a command runs in.
type Harness struct {
	Env    *env.Env
	Server *httptest.Server
	Store  *secret.Memory
	Out    *bytes.Buffer
	Err    *bytes.Buffer
	Stdin  *strings.Reader
	Now    time.Time
}

// Fixed clock used by every harness.
func fixedNow() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

// New starts handler on an httptest server, selects a context pointing at it and
// stores creds (kind -> value, kinds are secret.Kind*). Output defaults to json.
// stdin is what the command reads on its standard input.
func New(t *testing.T, handler http.Handler, creds map[string]string, stdin string) *Harness {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	h := &Harness{
		Server: srv, Store: secret.NewMemory(), Out: &bytes.Buffer{}, Err: &bytes.Buffer{},
		Stdin: strings.NewReader(stdin), Now: fixedNow(),
	}
	flags := &env.Flags{Context: "", Output: "json", Timeout: 10 * time.Second}
	h.Env = env.New(env.Deps{
		Stdin: h.Stdin, Stdout: h.Out, Stderr: h.Err,
		Clock:       func() time.Time { return h.Now },
		Getenv:      func(string) string { return "" },
		ConfigPath:  dir + "/config.json",
		Interactive: func() bool { return false },
		ReadPassword: func(string) (string, error) {
			t.Fatal("unexpected password prompt in test")
			return "", nil
		},
		OpenSecrets: func(secret.Options) (secret.Store, string, error) { return h.Store, "memory", nil },
		Version:     "test",
	}, flags)
	cf := &config.File{}
	if err := cf.Add(config.Context{Name: ContextName, URL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if err := h.Env.ConfigStore().Save(cf); err != nil {
		t.Fatal(err)
	}
	for kind, v := range creds {
		if err := h.Store.Set(secret.Name(ContextName, kind), v); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

// Session encodes a login session for the creds of New, valid for an hour past the
// harness clock, so no request triggers a refresh.
func Session(t *testing.T, accessToken string) string {
	t.Helper()
	s := auth.Session{AccessToken: accessToken, RefreshToken: "refresh-" + accessToken, ExpiresAt: fixedNow().Add(time.Hour)}
	enc, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

// Run executes cmd with args, returning its error. Output is in h.Out / h.Err.
func (h *Harness) Run(cmd *cobra.Command, args ...string) error {
	cmd.SetArgs(args)
	cmd.SetOut(h.Out)
	cmd.SetErr(h.Err)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	return cmd.Execute()
}
