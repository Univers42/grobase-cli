package orgs

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// inviteSpec is the POST /v1/orgs/{orgId}/invites body.
type inviteSpec struct {
	Email string `json:"email"`
	Role  string `json:"role,omitempty"`
}

func newInvitesCmd(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "invites",
		Short: "Manage the invitations of an organisation",
		Args:  exitcode.Args(cobra.NoArgs),
	}
	c.AddCommand(newInvitesListCmd(e), newInvitesCreateCmd(e))
	return c
}

func newInvitesListCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "list <orgId>",
		Short: "List the invitations of an organisation",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE:  func(cmd *cobra.Command, args []string) error { return runInvitesList(cmd.Context(), e, args[0]) },
	}
}

func runInvitesList(ctx context.Context, e *env.Env, orgID string) error {
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	var raw json.RawMessage
	req := httpx.Request{Method: http.MethodGet, Path: orgPath(orgID, "invites")}
	if err := cl.JSON(ctx, req, &raw); err != nil {
		return err
	}
	rows, err := decodeRows(raw, "invites")
	if err != nil {
		return err
	}
	return e.Printer().Print(rows, "id", "email", "role", "status", "expires_at")
}

func newInvitesCreateCmd(e *env.Env) *cobra.Command {
	var email, role string
	cmd := &cobra.Command{
		Use:   "create <orgId>",
		Short: "Invite someone to an organisation",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInvitesCreate(cmd.Context(), e, args[0], email, role)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&email, "email", "", "email address to invite (required)")
	fl.StringVar(&role, "role", "", "role to grant, e.g. developer")
	return cmd
}

func runInvitesCreate(ctx context.Context, e *env.Env, orgID, email, role string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return exitcode.Usagef("--email is required")
	}
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	var out map[string]any
	body := inviteSpec{Email: email, Role: strings.TrimSpace(role)}
	req := httpx.Request{Method: http.MethodPost, Path: orgPath(orgID, "invites"), JSON: body}
	if err := cl.JSON(ctx, req, &out); err != nil {
		return err
	}
	warnOnceInviteToken(e, out)
	return printResult(e, out)
}

// warnOnceInviteToken reminds the caller that the cleartext token comes back
// exactly once, at issue time; every later read is a redacted projection.
func warnOnceInviteToken(e *env.Env, out map[string]any) {
	if _, ok := out["token"]; ok {
		e.Warnf("store this invite token now; it is shown only once")
	}
}
