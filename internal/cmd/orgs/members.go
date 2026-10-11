package orgs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

func newMembersCmd(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "members",
		Short: "Manage the members of an organisation",
		Args:  exitcode.Args(cobra.NoArgs),
	}
	c.AddCommand(newMembersListCmd(e), newMembersRemoveCmd(e))
	return c
}

func newMembersListCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "list <orgId>",
		Short: "List the members of an organisation",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE:  func(cmd *cobra.Command, args []string) error { return runMembersList(cmd.Context(), e, args[0]) },
	}
}

func runMembersList(ctx context.Context, e *env.Env, orgID string) error {
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	var raw json.RawMessage
	req := httpx.Request{Method: http.MethodGet, Path: orgPath(orgID, "members")}
	if err := cl.JSON(ctx, req, &raw); err != nil {
		return err
	}
	rows, err := decodeRows(raw, "members")
	if err != nil {
		return err
	}
	return e.Printer().Print(rows, "user_id", "role", "created_at")
}

func newMembersRemoveCmd(e *env.Env) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <orgId> <userId>",
		Short: "Remove a member from an organisation",
		Args:  exitcode.Args(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMembersRemove(cmd.Context(), e, args[0], args[1], yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func runMembersRemove(ctx context.Context, e *env.Env, orgID, userID string, yes bool) error {
	if err := confirmDestructive(e, yes, fmt.Sprintf("remove %s from org %s?", userID, orgID)); err != nil {
		return err
	}
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	var out map[string]any
	req := httpx.Request{Method: http.MethodDelete, Path: orgPath(orgID, "members", userID)}
	if err := cl.JSON(ctx, req, &out); err != nil {
		return err
	}
	return printResult(e, out)
}
