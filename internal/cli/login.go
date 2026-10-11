package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/auth"
	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
	"github.com/Univers42/grobase-cli/internal/secret"
)

type loginFlags struct {
	email               string
	passwordStdin, otp  bool
	serviceRole, apiKey bool
	anonKeyStdin        bool
}

func newLoginCmd(e *env.Env) *cobra.Command {
	var f loginFlags
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against the current context and store the credential encrypted",
		Long: "Methods (pick one):\n" +
			"  --email E             password from a prompt, --password-stdin or $GROBASE_PASSWORD\n" +
			"  --email E --otp       emailed one-time code / magic link, code from a prompt or stdin\n" +
			"  --service-role        service-role key read from stdin (admin commands)\n" +
			"  --api-key             app API key (mbk_...) read from stdin\n\n" +
			"The anon key is needed by GoTrue: --anon-key-stdin (read first, before any other stdin\n" +
			"secret) or $GROBASE_ANON_KEY. Secrets are never accepted as flag values.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runLogin(cmd.Context(), e, f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.email, "email", "", "account email")
	fl.BoolVar(&f.passwordStdin, "password-stdin", false, "read the password from stdin")
	fl.BoolVar(&f.otp, "otp", false, "log in with an emailed one-time code instead of a password")
	fl.BoolVar(&f.serviceRole, "service-role", false, "store a service-role key read from stdin")
	fl.BoolVar(&f.apiKey, "api-key", false, "store an app API key read from stdin")
	fl.BoolVar(&f.anonKeyStdin, "anon-key-stdin", false, "read the anon (publishable) key from stdin")
	return cmd
}

func runLogin(ctx context.Context, e *env.Env, f loginFlags) error {
	c, err := e.Current()
	if err != nil {
		return err
	}
	store, err := e.Secrets()
	if err != nil {
		return err
	}
	if err := storeAnonKey(e, store, c.Name, f.anonKeyStdin); err != nil {
		return err
	}
	switch {
	case f.serviceRole:
		return loginStaticKey(ctx, e, store, c, secret.KindServiceRole, "Service-role key: ", "/auth/v1/admin/users?per_page=1", httpx.ModeService)
	case f.apiKey:
		return loginStaticKey(ctx, e, store, c, secret.KindAPIKey, "API key: ", "/v1/tenants/me", httpx.ModeAPIKey)
	case f.email != "":
		return loginEmail(ctx, e, store, c, f)
	}
	return exitcode.Usagef("choose a login method: --email, --service-role or --api-key")
}

// storeAnonKey saves a supplied anon key; without one the stored key is kept.
func storeAnonKey(e *env.Env, store secret.Store, ctxName string, fromStdin bool) error {
	if !fromStdin && e.Getenv("GROBASE_ANON_KEY") == "" {
		return nil
	}
	k, err := e.ReadSecret("Anon key: ", fromStdin, "GROBASE_ANON_KEY")
	if err != nil {
		return err
	}
	return store.Set(secret.Name(ctxName, secret.KindAnon), strings.TrimSpace(k))
}

func loginStaticKey(ctx context.Context, e *env.Env, store secret.Store, c config.Context, kind, prompt, probe string, mode httpx.Mode) error {
	key, err := e.ReadSecret(prompt, true, "")
	if err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return exitcode.Usagef("empty key on stdin")
	}
	if err := store.Set(secret.Name(c.Name, kind), key); err != nil {
		return err
	}
	cl, err := e.HTTPClient(c, true)
	if err != nil {
		return err
	}
	var apiErr *httpx.APIError
	verr := cl.JSON(ctx, httpx.Request{Method: "GET", Path: probe, Mode: mode}, nil)
	switch {
	case verr == nil:
	case errors.As(verr, &apiErr) && apiErr.Unauthorized():
		return errors.Join(fmt.Errorf("the server rejected this key: %w", verr), store.Delete(secret.Name(c.Name, kind)))
	default:
		e.Warnf("key stored but could not be verified: %v", verr)
	}
	_, err = fmt.Fprintf(e.Stdout, "logged in to %q with a %s\n", c.Name, strings.ReplaceAll(kind, "_", " "))
	return err
}

func loginEmail(ctx context.Context, e *env.Env, store secret.Store, c config.Context, f loginFlags) error {
	anon, err := store.Get(secret.Name(c.Name, secret.KindAnon))
	if err != nil {
		return exitcode.Usagef("no anon key stored for %q: pass --anon-key-stdin or set GROBASE_ANON_KEY", c.Name)
	}
	e.Redact.Add(anon)
	plain, err := e.HTTPClient(c, false)
	if err != nil {
		return err
	}
	g := &auth.GoTrue{Client: plain, Clock: e.Clock}
	var sess auth.Session
	if f.otp {
		sess, err = otpLogin(ctx, e, g, anon, f.email)
	} else {
		var pw string
		if pw, err = e.ReadSecret("Password: ", f.passwordStdin, "GROBASE_PASSWORD"); err == nil {
			sess, err = g.Password(ctx, anon, f.email, pw)
		}
	}
	if err != nil {
		return err
	}
	enc, err := sess.Encode()
	if err != nil {
		return err
	}
	if err := store.Set(secret.Name(c.Name, secret.KindSession), enc); err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.Stdout, "logged in to %q as %s\n", c.Name, sess.Email)
	return err
}

func otpLogin(ctx context.Context, e *env.Env, g *auth.GoTrue, anon, email string) (auth.Session, error) {
	if err := g.SendOTP(ctx, anon, email); err != nil {
		return auth.Session{}, err
	}
	_, _ = fmt.Fprintln(e.Stderr, "A one-time code was sent to the address if it exists.")
	code, err := e.ReadSecret("One-time code: ", !e.Interactive(), "GROBASE_OTP")
	if err != nil {
		return auth.Session{}, err
	}
	return g.VerifyOTP(ctx, anon, email, strings.TrimSpace(code))
}
