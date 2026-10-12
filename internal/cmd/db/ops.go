package db

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

func newTestCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "test <id>",
		Short: "Prove one mount answers: the query service opens it and lists its tables",
		Long: "Ask the query service for the mount's tables (GET /query/v1/{id}/tables): it\n" +
			"resolves the mount under the caller's tenant, decrypts its credentials and opens\n" +
			"the engine, so an answer proves the whole data path. A 404 means the mount does\n" +
			"not exist for this tenant.",
		Args: exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error { return runTest(cmd.Context(), e, args[0]) },
	}
}

// runTest reads the mount's tables. grobase serves no connection-test route: the
// POST /v1/tenants/me/mounts/{id}/test this command used to send was a 404 on every
// grobase (b2b data center, 2026-10-12), while the tables listing exercises the same
// connection through the data plane.
func runTest(ctx context.Context, e *env.Env, id string) error {
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	var out map[string]any
	req := httpx.Request{Method: http.MethodGet, Path: "/query/v1/" + httpx.Seg(id) + "/tables"}
	if err := cl.JSON(ctx, req, &out); err != nil {
		return err
	}
	return printResult(e, out)
}

func newRemoveCmd(e *env.Env) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Delete a database mount of the current tenant",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE:  func(cmd *cobra.Command, args []string) error { return runRemove(cmd.Context(), e, args[0], yes) },
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func runRemove(ctx context.Context, e *env.Env, id string, yes bool) error {
	if err := confirmDestructive(e, yes, fmt.Sprintf("delete mount %s?", id)); err != nil {
		return err
	}
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	var out map[string]any
	req := httpx.Request{Method: http.MethodDelete, Path: mountsPath(id)}
	if err := cl.JSON(ctx, req, &out); err != nil {
		return err
	}
	return printResult(e, out)
}
