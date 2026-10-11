// Package apps implements the `grobase apps` command.
package apps

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

// New returns the `apps` command.
func New(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "apps",
		Short: "Manage the account's self-serve apps",
		Long: "Each app is a new tenant with its own database and a scoped API key, shown once when it\n" +
			"is created; store it immediately. Apps need `grobase login` and a server with self-serve\n" +
			"apps enabled.",
	}
	c.AddCommand(newListCmd(e), newCreateCmd(e), newDeleteCmd(e))
	return c
}

// newListCmd builds `apps list`.
func newListCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the account's apps",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			rows, err := listApps(cmd.Context(), cl)
			if err != nil {
				return err
			}
			return e.Printer().Print(rows, "app_id", "name", "status", "created_at")
		},
	}
}

// newCreateCmd builds `apps create NAME [--engine ENGINE]`.
func newCreateCmd(e *env.Env) *cobra.Command {
	var engine string
	cmd := &cobra.Command{
		Use:   "create NAME [--engine ENGINE]",
		Short: "Create an app (its API key is printed once)",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			resp, err := createApp(cmd.Context(), cl, args[0], engine)
			if err != nil {
				return err
			}
			return printNewApp(e, resp)
		},
	}
	cmd.Flags().StringVar(&engine, "engine", "", "database engine for the app (default: the server's)")
	return cmd
}

// newDeleteCmd builds `apps delete APP_ID [--yes]`.
func newDeleteCmd(e *env.Env) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete APP_ID",
		Short: "Delete an app and the database it owns",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if err := e.ConfirmDestructive(yes, "delete app "+id); err != nil {
				return err
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			if err := deleteApp(cmd.Context(), cl, id); err != nil {
				return err
			}
			return e.Printer().Print(map[string]any{"deleted": id})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}
