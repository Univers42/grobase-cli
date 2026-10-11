package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/auth"
	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
)

type signupFlags struct {
	email                       string
	passwordStdin, anonKeyStdin bool
}

// newSignupCmd builds `grobase signup`: a new account, signed in when the
// deployment auto-confirms emails.
func newSignupCmd(e *env.Env) *cobra.Command {
	var f signupFlags
	cmd := &cobra.Command{
		Use:   "signup --email EMAIL",
		Short: "Create an account and sign in when the deployment auto-confirms emails",
		Long: "The anon key is needed by GoTrue: --anon-key-stdin (read first, before the password)\n" +
			"or $GROBASE_ANON_KEY. Secrets are never accepted as flag values.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runSignup(cmd.Context(), e, f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.email, "email", "", "account email (required)")
	fl.BoolVar(&f.passwordStdin, "password-stdin", false, "read the password from stdin")
	fl.BoolVar(&f.anonKeyStdin, "anon-key-stdin", false, "read the anon (publishable) key from stdin")
	return cmd
}

// runSignup asks GoTrue for an account and either stores the session it got back
// or tells the user to confirm the address first.
func runSignup(ctx context.Context, e *env.Env, f signupFlags) error {
	if f.email == "" {
		return exitcode.Usagef("--email is required")
	}
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
	sess, signedIn, err := createAccount(ctx, e, store, c, f)
	if err != nil {
		return err
	}
	if !signedIn {
		_, err = fmt.Fprintf(e.Stdout, "account created for %s; confirm the email, then: grobase login --email %s\n", f.email, f.email)
		return err
	}
	if err := storeSession(store, c, sess); err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.Stdout, "signed up and logged in to %q as %s\n", c.Name, sess.Email)
	return err
}

// createAccount reads the password and posts the signup with the stored anon key.
func createAccount(ctx context.Context, e *env.Env, store secret.Store, c config.Context, f signupFlags) (auth.Session, bool, error) {
	anon, err := storedAnonKey(e, store, c.Name)
	if err != nil {
		return auth.Session{}, false, err
	}
	pw, err := e.ReadSecret("Password: ", f.passwordStdin, "GROBASE_PASSWORD")
	if err != nil {
		return auth.Session{}, false, err
	}
	plain, err := e.HTTPClient(c, false)
	if err != nil {
		return auth.Session{}, false, err
	}
	g := &auth.GoTrue{Client: plain, Clock: e.Clock}
	return g.Signup(ctx, anon, f.email, pw)
}
