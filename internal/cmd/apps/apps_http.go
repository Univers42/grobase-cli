package apps

import (
	"context"
	"net/http"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// appsPath is the account's apps collection. It answers 404 when the deployment
// has self-serve apps switched off.
const appsPath = "/v1/tenants/me/apps"

// listApps GETs the account's apps. A 404 on the collection root means the
// feature is disabled, not that the account has none.
func listApps(ctx context.Context, cl *httpx.Client) ([]any, error) {
	var rows []any
	req := httpx.Request{Method: http.MethodGet, Path: appsPath, Mode: httpx.ModeUser}
	if err := cl.JSON(ctx, req, &rows); err != nil {
		return nil, httpx.NotEnabled("apps", err)
	}
	return rows, nil
}

// appSpec is the POST body: Engine is left out when none was picked, so the
// server applies its own default.
type appSpec struct {
	Name   string `json:"name"`
	Engine string `json:"engine,omitempty"`
}

// createApp POSTs a new app and returns the raw response, cleartext key included.
func createApp(ctx context.Context, cl *httpx.Client, name, engine string) (map[string]any, error) {
	var resp map[string]any
	req := httpx.Request{Method: http.MethodPost, Path: appsPath, JSON: appSpec{Name: name, Engine: engine}, Mode: httpx.ModeUser}
	if err := cl.JSON(ctx, req, &resp); err != nil {
		return nil, httpx.NotEnabled("apps", err)
	}
	return resp, nil
}

// printNewApp prints a freshly created app and, while the answer still holds its
// cleartext key, warns that it will not be shown again.
func printNewApp(e *env.Env, resp map[string]any) error {
	if err := e.Printer().Print(resp); err != nil {
		return err
	}
	if key, _ := resp["api_key"].(string); key != "" {
		e.Warnf("store this key now; it cannot be shown again")
	}
	return nil
}

// deleteApp removes id. A 404 on a specific id is a genuine "not found", so this
// error is returned untouched (only the collection root is relabelled).
func deleteApp(ctx context.Context, cl *httpx.Client, id string) error {
	req := httpx.Request{Method: http.MethodDelete, Path: appsPath + "/" + httpx.Seg(id), Mode: httpx.ModeUser}
	return cl.JSON(ctx, req, nil)
}
