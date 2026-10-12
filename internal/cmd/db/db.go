// Package db implements the `grobase db` command.
package db

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// New returns the `db` command.
func New(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "db",
		Short: "Manage the database mounts of the current tenant",
		Args:  exitcode.Args(cobra.NoArgs),
	}
	c.AddCommand(newListCmd(e), newAddCmd(e), newTestCmd(e), newRemoveCmd(e), newMigrateCmd(e))
	return c
}

// mountsPath builds the tenant-scoped mounts path, escaping every supplied id.
func mountsPath(id ...string) string {
	p := "/v1/tenants/me/mounts"
	for _, s := range id {
		p += "/" + httpx.Seg(s)
	}
	return p
}

// decodeRows accepts both list shapes grobase uses: a bare JSON array or an
// object wrapping it under key.
func decodeRows(raw json.RawMessage, key string) ([]map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return []map[string]any{}, nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err == nil {
		return nonNil(rows), nil
	}
	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("decode list: %w", err)
	}
	inner, ok := wrapped[key]
	if !ok {
		return []map[string]any{}, nil
	}
	if err := json.Unmarshal(inner, &rows); err != nil {
		return nil, fmt.Errorf("decode %s: %w", key, err)
	}
	return nonNil(rows), nil
}

func nonNil(rows []map[string]any) []map[string]any {
	if rows == nil {
		return []map[string]any{}
	}
	return rows
}

// printResult shows the server's answer, or a bare confirmation when the route
// replied with an empty body, so a successful command is never silent.
func printResult(e *env.Env, out map[string]any) error {
	if len(out) > 0 {
		return e.Printer().Print(out)
	}
	return e.Printer().Print(map[string]any{"ok": true})
}

// withoutSecrets drops the fields a deployment may echo back on a mount:
// printing one would put a credential in the scrollback.
func withoutSecrets(m map[string]any) map[string]any {
	for _, k := range []string{"connection_string", "dsn"} {
		delete(m, k)
	}
	return m
}

// confirmDestructive gates a destructive command: --yes skips the question, any
// other answer aborts before a single byte reaches the server.
func confirmDestructive(e *env.Env, yes bool, question string) error {
	if yes {
		return nil
	}
	ok, err := e.Confirm(question)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted: nothing was deleted")
	}
	return nil
}
