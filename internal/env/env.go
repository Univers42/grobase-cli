// Package env carries everything a command needs: injected I/O, clock, config and
// credential stores, and the helpers that build an authenticated HTTP client.
// Commands receive an *Env from their constructor; nothing is read from globals.
package env

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Univers42/grobase-cli/internal/auth"
	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
	"github.com/Univers42/grobase-cli/internal/output"
	"github.com/Univers42/grobase-cli/internal/redact"
	"github.com/Univers42/grobase-cli/internal/secret"
)

// Deps are the injected side effects.
type Deps struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Clock  func() time.Time
	Getenv func(string) string
	// ConfigPath is the config file; the encrypted credential file sits beside it.
	ConfigPath string
	// Interactive reports whether Stdin is a terminal.
	Interactive func() bool
	// ReadPassword prompts on the terminal without echo.
	ReadPassword func(prompt string) (string, error)
	// OpenSecrets builds the credential store (tests inject a memory store).
	OpenSecrets func(secret.Options) (secret.Store, string, error)
	// KDF overrides the argon2id cost (tests); zero means secret.DefaultKDF.
	KDF secret.KDF
	// Version is shown in the User-Agent.
	Version string
}

// Flags are the root persistent flags, bound by the root command.
type Flags struct {
	Context         string
	Output          string
	JQ              string
	Debug           bool
	Timeout         time.Duration
	CredentialStore string
}

// Env is the per-invocation environment shared by all commands.
type Env struct {
	Deps
	Flags  *Flags
	Redact *redact.Redactor

	once   sync.Once
	store  secret.Store
	err    error
	stdin  *bufio.Reader
	mu     sync.Mutex
	picked map[string]string
}

// New returns an Env over deps and flags.
func New(deps Deps, flags *Flags) *Env {
	return &Env{Deps: deps, Flags: flags, Redact: redact.New()}
}

// ConfigStore returns the config file store.
func (e *Env) ConfigStore() config.Store { return config.Store{Path: e.ConfigPath} }

// Printer returns the output printer honouring -o and --jq.
func (e *Env) Printer() output.Printer {
	return output.Printer{W: e.Stdout, Format: e.Flags.Output, JQ: e.Flags.JQ}
}

// Warnf prints a warning to stderr.
func (e *Env) Warnf(format string, a ...any) {
	_, _ = fmt.Fprintf(e.Stderr, "warning: "+format+"\n", a...)
}

// Current loads the selected context (the --context flag wins over the config file).
func (e *Env) Current() (config.Context, error) {
	f, err := e.ConfigStore().Load()
	if err != nil {
		return config.Context{}, err
	}
	return f.Get(e.Flags.Context)
}

// Secrets opens the credential store once.
func (e *Env) Secrets() (secret.Store, error) {
	e.once.Do(func() {
		kdf := e.KDF
		if kdf == (secret.KDF{}) {
			kdf = secret.DefaultKDF()
		}
		open := e.OpenSecrets
		if open == nil {
			open = secret.Open
		}
		e.store, _, e.err = open(secret.Options{
			Backend:    e.Flags.CredentialStore,
			FilePath:   filepath.Join(filepath.Dir(e.ConfigPath), "credentials.enc"),
			KDF:        kdf,
			Passphrase: e.passphrase,
		})
	})
	return e.store, e.err
}

// passphrase returns the credential-file passphrase from the environment or a prompt.
func (e *Env) passphrase() ([]byte, error) {
	if p := e.Getenv("GROBASE_PASSPHRASE"); p != "" {
		return []byte(p), nil
	}
	if !e.Interactive() {
		return nil, errors.New("credential file needs a passphrase: set GROBASE_PASSPHRASE or run interactively")
	}
	p, err := e.ReadPassword("Credential file passphrase: ")
	return []byte(p), err
}

// HTTPClient builds a client for ctx. With withAuth the client injects stored credentials.
func (e *Env) HTTPClient(c config.Context, withAuth bool) (*httpx.Client, error) {
	target, err := e.SelectedURL(c)
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("context %q: %w", c.Name, err)
	}
	tr, err := httpx.NewTransport(httpx.TLS{CAFile: c.CAFile, PinSHA256: c.PinSHA256, Insecure: c.Insecure}, e.Flags.Timeout)
	if err != nil {
		return nil, err
	}
	if c.Insecure {
		e.Warnf("TLS verification is DISABLED for context %q; traffic can be intercepted", c.Name)
	}
	cl := &httpx.Client{Base: base, HTTP: &http.Client{Transport: tr}, Redact: e.Redact, UserAgent: "grobase-cli/" + e.Version}
	if e.Flags.Debug {
		cl.Debug = e.Stderr
	}
	if withAuth {
		store, err := e.Secrets()
		if err != nil {
			return nil, err
		}
		plain := *cl
		gate := &auth.GoTrue{Client: &plain, Clock: e.Clock}
		cl.Auth = &auth.Resolver{Context: c.Name, Store: store, Clock: e.Clock, Redact: e.Redact, Gate: gate}
	}
	return cl, nil
}

// Client returns an authenticated client for the current context.
func (e *Env) Client() (*httpx.Client, config.Context, error) {
	c, err := e.Current()
	if err != nil {
		return nil, c, err
	}
	cl, err := e.HTTPClient(c, true)
	return cl, c, err
}

// ReadSecret obtains a secret without ever taking it from argv: stdin when fromStdin,
// else the env var, else a no-echo terminal prompt.
func (e *Env) ReadSecret(prompt string, fromStdin bool, envVar string) (string, error) {
	v, err := e.readSecret(prompt, fromStdin, envVar)
	e.Redact.Add(v)
	return v, err
}

// readSecret resolves one secret from stdin, the environment or a prompt.
func (e *Env) readSecret(prompt string, fromStdin bool, envVar string) (string, error) {
	switch {
	case fromStdin:
		line, err := e.ReadLine()
		return strings.TrimRight(line, "\r\n"), err
	case envVar != "" && e.Getenv(envVar) != "":
		return e.Getenv(envVar), nil
	case e.Interactive():
		return e.ReadPassword(prompt)
	}
	return "", &exitcode.UsageError{Msg: fmt.Sprintf("no secret available: pass it on stdin (--*-stdin), set %s, or run in a terminal", envVar)}
}

// ReadLine reads one line from stdin (shared buffer so successive calls compose).
func (e *Env) ReadLine() (string, error) {
	if e.stdin == nil {
		e.stdin = bufio.NewReader(e.Stdin)
	}
	line, err := e.stdin.ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// ConfirmDestructive gates a destructive action: --yes skips the prompt. A
// non-interactive caller must pass --yes; a declined prompt cancels as a usage
// error, so nothing was sent to the server.
func (e *Env) ConfirmDestructive(yes bool, action string) error {
	if yes {
		return nil
	}
	if !e.Interactive() {
		return exitcode.Usagef("pass --yes to confirm: %s", action)
	}
	ok, err := e.Confirm(action + "? this cannot be undone")
	if err != nil {
		return err
	}
	if !ok {
		return exitcode.Usagef("cancelled; nothing was %s", action)
	}
	return nil
}

// Confirm asks a yes/no question on stderr and reads the answer from stdin.
func (e *Env) Confirm(question string) (bool, error) {
	_, _ = fmt.Fprintf(e.Stderr, "%s [y/N]: ", question)
	line, err := e.ReadLine()
	if err != nil {
		return false, err
	}
	a := strings.ToLower(strings.TrimSpace(line))
	return a == "y" || a == "yes", nil
}
