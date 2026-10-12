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

func newSessionsCmd(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "sessions",
		Short: "Handle the sessions of a user",
		Args:  exitcode.Args(cobra.NoArgs),
	}
	c.AddCommand(newSessionsRevokeCmd(e))
	return c
}

func newSessionsRevokeCmd(e *env.Env) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "revoke <id|email>",
		Short: "Sign a user out of every device",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessionsRevoke(cmd.Context(), e, args[0], yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "revoke without asking first")
	return cmd
}

func runSessionsRevoke(ctx context.Context, e *env.Env, id string, yes bool) error {
	// Revoking only signs the account out and the user may simply log in again, so a
	// terminal is asked while a script is trusted to mean it.
	if !yes && e.Interactive() {
		if err := confirm(e, "Revoke every session of user "+id+"?"); err != nil {
			return err
		}
	}
	cl, err := serviceClient(e)
	if err != nil {
		return err
	}
	if id, err = userID(ctx, cl, id); err != nil {
		return err
	}
	err = cl.JSON(ctx, adminReq(http.MethodPost, adminUsersPath+"/"+httpx.Seg(id)+"/logout"), nil)
	if err != nil {
		return httpx.NotEnabled("session revocation", err)
	}
	_, err = fmt.Fprintf(e.Stdout, "revoked every session of user %s\n", id)
	return err
}
