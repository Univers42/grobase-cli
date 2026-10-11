package users

import (
	"context"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// defaultBanDuration is a century: GoTrue's way of saying "until somebody unbans".
const defaultBanDuration = "876000h"

func newBanCmd(e *env.Env) *cobra.Command {
	var duration string
	cmd := &cobra.Command{
		Use:   "ban <id>",
		Short: "Ban a user",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBan(cmd.Context(), e, args[0], duration)
		},
	}
	cmd.Flags().StringVar(&duration, "duration", defaultBanDuration, "how long to ban, e.g. 24h or 876000h")
	return cmd
}

func runBan(ctx context.Context, e *env.Env, id, duration string) error {
	if _, err := time.ParseDuration(duration); err != nil {
		return exitcode.Usagef("--duration %q is not a duration such as 24h: %v", duration, err)
	}
	return setBan(ctx, e, id, duration)
}

func newUnbanCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "unban <id>",
		Short: "Lift the ban of a user",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setBan(cmd.Context(), e, args[0], "none")
		},
	}
}

func setBan(ctx context.Context, e *env.Env, id, banDuration string) error {
	cl, err := serviceClient(e)
	if err != nil {
		return err
	}
	r := adminReq(http.MethodPut, adminUsersPath+"/"+httpx.Seg(id))
	r.JSON = map[string]any{"ban_duration": banDuration}
	var u map[string]any
	if err := cl.JSON(ctx, r, &u); err != nil {
		return err
	}
	return printUser(e, u, "user "+id+" is now banned until further notice")
}
