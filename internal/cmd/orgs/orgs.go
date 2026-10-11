// Package orgs implements the `grobase orgs` command.
package orgs

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

// New returns the `orgs` command.
func New(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "orgs",
		Short: "Manage organisations, their members, invites and projects",
		Args:  exitcode.Args(cobra.NoArgs),
	}
	c.AddCommand(newListCmd(e), newShowCmd(e), newMembersCmd(e), newInvitesCmd(e),
		newProjectsCmd(e), newUsageCmd(e))
	return c
}

// orgPath builds an /v1/orgs path, escaping every user-supplied segment.
func orgPath(id ...string) string {
	p := "/v1/orgs"
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
