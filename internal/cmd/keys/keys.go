// Package keys implements the `grobase keys` command.
package keys

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

// New returns the `keys` command.
func New(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "keys",
		Short: "Manage the current tenant's API keys",
		Long: "API keys authenticate machine clients. A key's cleartext value is shown\n" +
			"only once, when it is created or rotated; store it immediately.",
	}
	c.AddCommand(newListCmd(e), newCreateCmd(e), newRevokeCmd(e), newRotateCmd(e))
	return c
}

func newListCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the current tenant's API keys",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			ks, err := listKeys(cmd.Context(), cl)
			if err != nil {
				return err
			}
			return e.Printer().Print(ks, "id", "name", "scopes", "created_at", "revoked_at")
		},
	}
}

func newCreateCmd(e *env.Env) *cobra.Command {
	var name string
	var scopes []string
	cmd := &cobra.Command{
		Use:   "create --name NAME [--scope SCOPE ...]",
		Short: "Create an API key (its cleartext is printed once)",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return exitcode.Usagef("--name is required")
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			resp, err := createKey(cmd.Context(), cl, name, scopes)
			if err != nil {
				return err
			}
			return printNewKey(e, resp)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "human-readable name for the key")
	cmd.Flags().StringSliceVar(&scopes, "scope", nil, "restrict the key to a scope (repeatable)")
	return cmd
}

func newRevokeCmd(e *env.Env) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "revoke ID",
		Short: "Revoke an API key",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := gate(e, yes, "revoke key "+args[0]); err != nil {
				return err
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			if err := deleteKey(cmd.Context(), cl, args[0]); err != nil {
				return err
			}
			return e.Printer().Print(map[string]any{"revoked": args[0]})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func newRotateCmd(e *env.Env) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "rotate ID",
		Short: "Create a replacement key, print it, then revoke the old one",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := gate(e, yes, "rotate key "+id); err != nil {
				return err
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			name, scopes, err := findKey(cmd.Context(), cl, id)
			if err != nil {
				return err
			}
			resp, err := createKey(cmd.Context(), cl, name, scopes)
			if err != nil {
				return err
			}
			if err := printNewKey(e, resp); err != nil {
				return err
			}
			// Only after a usable replacement exists is the old key revoked.
			return deleteKey(cmd.Context(), cl, id)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}
