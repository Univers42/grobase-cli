package users

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

func newDeleteCmd(e *env.Env) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a user permanently",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd.Context(), e, args[0], yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "delete without asking first")
	return cmd
}

func runDelete(ctx context.Context, e *env.Env, id string, yes bool) error {
	// Deleting is irreversible, so it needs an explicit answer: --yes, or a terminal.
	if err := confirmAlways(e, yes, "Delete user "+id+" permanently?"); err != nil {
		return err
	}
	cl, err := serviceClient(e)
	if err != nil {
		return err
	}
	var u map[string]any
	if err := cl.JSON(ctx, adminReq(http.MethodDelete, adminUsersPath+"/"+httpx.Seg(id)), &u); err != nil {
		return err
	}
	return printUser(e, u, "deleted user "+id)
}

// confirmAlways gates an irreversible action: --yes skips the question and a
// non-interactive caller has nobody to ask, so it must spell --yes.
func confirmAlways(e *env.Env, yes bool, question string) error {
	if yes {
		return nil
	}
	if !e.Interactive() {
		return exitcode.Usagef("refusing to continue: pass --yes")
	}
	return confirm(e, question)
}

// confirm asks the question and reports a negative answer as a usage error.
func confirm(e *env.Env, question string) error {
	ok, err := e.Confirm(question)
	if err != nil {
		return err
	}
	if !ok {
		return exitcode.Usagef("cancelled")
	}
	return nil
}

// printUser renders the user object the server sent, or a plain line when it sent none.
func printUser(e *env.Env, u map[string]any, fallback string) error {
	if len(u) == 0 {
		_, err := fmt.Fprintln(e.Stdout, fallback)
		return err
	}
	return e.Printer().Print(u)
}
