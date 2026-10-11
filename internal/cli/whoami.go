package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/auth"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
	"github.com/Univers42/grobase-cli/internal/secret"
)

func present(store secret.Store, ctxName, kind string) bool {
	v, err := store.Get(secret.Name(ctxName, kind))
	return err == nil && v != ""
}

func newWhoamiCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use: "whoami", Short: "Show which credentials are stored and who the server thinks you are", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, c, err := e.Client()
			if err != nil {
				return err
			}
			store, err := e.Secrets()
			if err != nil {
				return err
			}
			out := map[string]any{"context": c.Name, "url": c.URL}
			for _, k := range []string{secret.KindAnon, secret.KindServiceRole, secret.KindAPIKey, secret.KindSession} {
				out[k] = present(store, c.Name, k)
			}
			if present(store, c.Name, secret.KindSession) {
				var user map[string]any
				err := cl.JSON(cmd.Context(), httpx.Request{Method: "GET", Path: "/auth/v1/user", Mode: httpx.ModeUser}, &user)
				switch {
				case err == nil:
					out["user"] = map[string]any{"id": user["id"], "email": user["email"], "role": user["role"]}
				case errors.Is(err, auth.ErrNoCredential):
					return err
				default:
					out["user_error"] = err.Error()
				}
			}
			return e.Printer().Print(out)
		},
	}
}

func newLogoutCmd(e *env.Env) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use: "logout", Short: "Forget the session (and with --all every stored key) of the current context", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, c, err := e.Client()
			if err != nil {
				return err
			}
			store, err := e.Secrets()
			if err != nil {
				return err
			}
			if present(store, c.Name, secret.KindSession) {
				// Best effort: the local session is dropped even if the server is unreachable.
				if err := cl.JSON(cmd.Context(), httpx.Request{Method: "POST", Path: "/auth/v1/logout", Mode: httpx.ModeUser}, nil); err != nil {
					e.Warnf("server-side logout failed: %v", err)
				}
			}
			kinds := []string{secret.KindSession}
			if all {
				kinds = []string{secret.KindSession, secret.KindServiceRole, secret.KindAPIKey, secret.KindAnon}
			}
			for _, k := range kinds {
				if err := store.Delete(secret.Name(c.Name, k)); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintf(e.Stdout, "logged out of %q\n", c.Name)
			return err
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "also delete the stored anon, service-role and API keys")
	return cmd
}

func newTokenCmd(e *env.Env) *cobra.Command {
	c := &cobra.Command{Use: "token", Short: "Handle raw credentials (explicit only)"}
	var kind string
	p := &cobra.Command{
		Use: "print", Short: "Print one stored credential to stdout (access, refresh, service-role, api-key, anon)",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cur, err := e.Current()
			if err != nil {
				return err
			}
			v, err := tokenValue(cmd, e, cur.Name, kind)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(e.Stdout, v)
			return err
		},
	}
	p.Flags().StringVar(&kind, "kind", "access", "which credential: access, refresh, service-role, api-key or anon")
	c.AddCommand(p)
	return c
}

func tokenValue(cmd *cobra.Command, e *env.Env, ctxName, kind string) (string, error) {
	store, err := e.Secrets()
	if err != nil {
		return "", err
	}
	switch kind {
	case "access", "refresh":
		cl, _, err := e.Client()
		if err != nil {
			return "", err
		}
		res, ok := cl.Auth.(*auth.Resolver)
		if !ok {
			return "", errors.New("internal: unexpected authorizer")
		}
		s, found, err := res.Session(cmd.Context())
		if err != nil {
			return "", err
		}
		if !found {
			return "", fmt.Errorf("%w: not logged in", auth.ErrNoCredential)
		}
		if kind == "refresh" {
			return s.RefreshToken, nil
		}
		return s.AccessToken, nil
	case "service-role", "api-key", "anon":
		v, err := store.Get(secret.Name(ctxName, map[string]string{"service-role": secret.KindServiceRole, "api-key": secret.KindAPIKey, "anon": secret.KindAnon}[kind]))
		if errors.Is(err, secret.ErrNotFound) {
			return "", fmt.Errorf("%w: no %s stored", auth.ErrNoCredential, kind)
		}
		return v, err
	}
	return "", exitcode.Usagef("unknown --kind %q", kind)
}
