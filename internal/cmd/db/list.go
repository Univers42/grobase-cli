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
	var tenant string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the database mounts of the current tenant, or of --tenant with the service-role key",
		Long: "Without --tenant, the mounts of the tenant the credential belongs to (tenant\n" +
			"self-service, /v1/tenants/me/mounts). With --tenant, any tenant's mounts through the\n" +
			"service-role admin route, which also works where self-service is switched off.",
		Args: exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error { return runList(cmd.Context(), e, tenant) },
	}
	cmd.Flags().StringVar(&tenant, "tenant", "", "tenant whose mounts to list (needs `grobase login --service-role`)")
	return cmd
}

func runList(ctx context.Context, e *env.Env, tenant string) error {
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	if tenant != "" {
		rows, err := tenantMounts(ctx, cl, tenant)
		if err != nil {
			return err
		}
		return e.Printer().Print(rows, "id", "name", "engine", "tenant_id", "last_healthy_at")
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
