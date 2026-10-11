package db

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

func newListCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the database mounts of the current tenant",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE:  func(cmd *cobra.Command, _ []string) error { return runList(cmd.Context(), e) },
	}
}

func runList(ctx context.Context, e *env.Env) error {
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	var raw json.RawMessage
	req := httpx.Request{Method: http.MethodGet, Path: mountsPath()}
	if err := cl.JSON(ctx, req, &raw); err != nil {
		return httpx.NotEnabled("database mounts", err)
	}
	rows, err := decodeRows(raw, "mounts")
	if err != nil {
		return err
	}
	return e.Printer().Print(rows, "id", "name", "engine", "isolation", "status")
}
