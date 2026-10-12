package db

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// adminMountsPath is the service-role listing of one tenant's mounts: the Go
// adapter-registry behind Kong's /admin/v1 (acl baas-admin), scoped by
// X-Baas-Tenant-Id. It works where /v1/tenants/me/mounts is switched off
// (TENANT_SELFSERVE_ENABLED), and it never returns a connection string.
const adminMountsPath = "/admin/v1/databases"

// tenantRequest is a service-role request on behalf of one tenant.
func tenantRequest(method, path, tenant string, body any) httpx.Request {
	return httpx.Request{
		Method: method, Path: path, Mode: httpx.ModeService, JSON: body,
		Header: http.Header{"X-Baas-Tenant-Id": {tenant}},
	}
}

// tenantMounts lists one tenant's mounts with the service-role key.
func tenantMounts(ctx context.Context, cl *httpx.Client, tenant string) ([]map[string]any, error) {
	var raw json.RawMessage
	if err := cl.JSON(ctx, tenantRequest(http.MethodGet, adminMountsPath, tenant, nil), &raw); err != nil {
		return nil, httpx.NotEnabled("tenant mounts", err)
	}
	rows, err := decodeRows(raw, "mounts")
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		withoutSecrets(r)
	}
	return rows, nil
}

// findMount picks the mount whose id or name is ref: an id wins, a name must be unique.
func findMount(rows []map[string]any, ref string) (map[string]any, error) {
	var byName []map[string]any
	for _, r := range rows {
		if field(r, "id") == ref {
			return r, nil
		}
		if field(r, "name") == ref {
			byName = append(byName, r)
		}
	}
	switch len(byName) {
	case 1:
		return byName[0], nil
	case 0:
		names := make([]string, 0, len(rows))
		for _, r := range rows {
			names = append(names, field(r, "name"))
		}
		sort.Strings(names)
		return nil, exitcode.NotFoundf("no mount %q for this tenant (its mounts: %s)", ref, strings.Join(names, ", "))
	}
	return nil, fmt.Errorf("%d mounts are named %q: pass the id instead", len(byName), ref)
}

// field reads one string field of a decoded row.
func field(row map[string]any, key string) string {
	s, _ := row[key].(string)
	return s
}
