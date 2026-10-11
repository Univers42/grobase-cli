package keys

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

const keysPath = "/v1/tenants/me/keys"

// listKeys GETs the tenant's keys; a 404 here means the feature is disabled.
func listKeys(ctx context.Context, cl *httpx.Client) ([]any, error) {
	var raw any
	req := httpx.Request{Method: "GET", Path: keysPath, Mode: httpx.ModeAuto}
	if err := cl.JSON(ctx, req, &raw); err != nil {
		return nil, httpx.NotEnabled("keys", err)
	}
	return extractKeys(raw)
}

// createKey POSTs a new key and returns the raw response (cleartext included).
func createKey(ctx context.Context, cl *httpx.Client, name string, scopes []string) (map[string]any, error) {
	body := map[string]any{"name": name}
	if len(scopes) > 0 {
		body["scopes"] = scopes
	}
	var resp map[string]any
	req := httpx.Request{Method: "POST", Path: keysPath, JSON: body, Mode: httpx.ModeAuto}
	if err := cl.JSON(ctx, req, &resp); err != nil {
		return nil, httpx.NotEnabled("keys", err)
	}
	return resp, nil
}

// deleteKey removes id. A 404 on a specific id is a genuine "not found", so this
// error is returned untouched (only the collection root is relabelled).
func deleteKey(ctx context.Context, cl *httpx.Client, id string) error {
	req := httpx.Request{Method: "DELETE", Path: keysPath + "/" + httpx.Seg(id), Mode: httpx.ModeAuto}
	return cl.JSON(ctx, req, nil)
}

// findKey locates id in the key list and returns its name and scopes for rotation.
func findKey(ctx context.Context, cl *httpx.Client, id string) (string, []string, error) {
	ks, err := listKeys(ctx, cl)
	if err != nil {
		return "", nil, err
	}
	for _, k := range ks {
		if m, ok := k.(map[string]any); ok && m["id"] == id {
			return stringField(m, "name"), stringSlice(m, "scopes"), nil
		}
	}
	return "", nil, fmt.Errorf("key %s: %w", id, fs.ErrNotExist)
}

// printNewKey prints a freshly created key's response, then warns that the
// cleartext will not be shown again.
func printNewKey(e *env.Env, resp map[string]any) error {
	if err := e.Printer().Print(resp); err != nil {
		return err
	}
	e.Warnf("store this key now; it cannot be shown again")
	return nil
}

// gate confirms a destructive action: --yes skips the prompt. A non-interactive
// caller must pass --yes; a declined prompt cancels as a usage error.
func gate(e *env.Env, yes bool, action string) error {
	if yes {
		return nil
	}
	if !e.Interactive() {
		return exitcode.Usagef("pass --yes to confirm: %s", action)
	}
	ok, err := e.Confirm(action + "? this cannot be undone")
	if err != nil {
		return err
	}
	if !ok {
		return exitcode.Usagef("cancelled; nothing was %s", action)
	}
	return nil
}

// extractKeys accepts either a bare array of keys or an object wrapping them
// under "keys"; both shapes have shipped from the control plane.
func extractKeys(raw any) ([]any, error) {
	switch t := raw.(type) {
	case []any:
		return t, nil
	case map[string]any:
		if ks, ok := t["keys"].([]any); ok {
			return ks, nil
		}
	}
	return nil, fmt.Errorf("unexpected keys response: %T (want an array or {\"keys\":[...]})", raw)
}

func stringField(m map[string]any, k string) string {
	if s, ok := m[k].(string); ok {
		return s
	}
	return ""
}

func stringSlice(m map[string]any, k string) []string {
	raw, ok := m[k].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
