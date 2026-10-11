package orgs

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
		Short: "List the organisations you belong to",
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
	req := httpx.Request{Method: http.MethodGet, Path: orgPath()}
	if err := cl.JSON(ctx, req, &raw); err != nil {
		return httpx.NotEnabled("orgs", err)
	}
	rows, err := decodeRows(raw, "orgs")
	if err != nil {
		return err
	}
	return e.Printer().Print(rows, "id", "slug", "name", "plan", "status")
}

func newShowCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "show <orgId>",
		Short: "Show one organisation",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE:  func(cmd *cobra.Command, args []string) error { return runShow(cmd.Context(), e, args[0]) },
	}
}

func runShow(ctx context.Context, e *env.Env, orgID string) error {
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	var out map[string]any
	req := httpx.Request{Method: http.MethodGet, Path: orgPath(orgID)}
	if err := cl.JSON(ctx, req, &out); err != nil {
		return err
	}
	return printResult(e, out)
}

func newProjectsCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "projects <orgId>",
		Short: "List the projects attached to an organisation",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE:  func(cmd *cobra.Command, args []string) error { return runProjects(cmd.Context(), e, args[0]) },
	}
}

func runProjects(ctx context.Context, e *env.Env, orgID string) error {
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	var raw json.RawMessage
	req := httpx.Request{Method: http.MethodGet, Path: orgPath(orgID, "projects")}
	if err := cl.JSON(ctx, req, &raw); err != nil {
		return err
	}
	rows, err := decodeRows(raw, "projects")
	if err != nil {
		return err
	}
	return e.Printer().Print(rows, "slug", "name", "plan", "status")
}

func newUsageCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "usage <orgId>",
		Short: "Show the metered usage rollup of an organisation",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE:  func(cmd *cobra.Command, args []string) error { return runUsage(cmd.Context(), e, args[0]) },
	}
}

func runUsage(ctx context.Context, e *env.Env, orgID string) error {
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	// The rollup is one object: org_id, the per-metric rows and their sum, which
	// the printer renders as a single record whatever the output format.
	var out map[string]any
	req := httpx.Request{Method: http.MethodGet, Path: orgPath(orgID, "usage")}
	if err := cl.JSON(ctx, req, &out); err != nil {
		return err
	}
	return printResult(e, out)
}
