// Package tenants implements the `grobase tenants` command.
package tenants

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// New returns the `tenants` command.
func New(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "tenants",
		Short: "Show the current tenant's profile, resource usage and entitlements",
	}
	c.AddCommand(newShowCmd(e), newUsageCmd(e), newEntitlementsCmd(e))
	return c
}

func newShowCmd(e *env.Env) *cobra.Command {
	return getCmd(e, "show", "Show the current tenant", "/v1/tenants/me")
}

func newUsageCmd(e *env.Env) *cobra.Command {
	return getCmd(e, "usage", "Show the current tenant's resource usage", "/v1/tenants/me/usage")
}

func newEntitlementsCmd(e *env.Env) *cobra.Command {
	return getCmd(e, "entitlements", "Show the current tenant's entitlements", "/v1/tenants/me/entitlements")
}

// getCmd builds a read-only subcommand that GETs one tenant route and prints it.
func getCmd(e *env.Env, use, short, path string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return get(cmd.Context(), e, use, path)
		},
	}
}

func get(ctx context.Context, e *env.Env, what, path string) error {
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	var out any
	if err := cl.JSON(ctx, httpx.Request{Method: "GET", Path: path, Mode: httpx.ModeAuto}, &out); err != nil {
		return httpx.NotEnabled("tenants "+what, err)
	}
	return e.Printer().Print(out)
}
