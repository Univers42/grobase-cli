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
		Short: "Ask the server to test the connection of one mount",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE:  func(cmd *cobra.Command, args []string) error { return runTest(cmd.Context(), e, args[0]) },
	}
}

func runTest(ctx context.Context, e *env.Env, id string) error {
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	var out map[string]any
	req := httpx.Request{Method: http.MethodPost, Path: mountsPath(id, "test")}
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
