package users

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

type createFlags struct {
	email, role, metadata  string
	passwordStdin, confirm bool
}

func newCreateCmd(e *env.Env) *cobra.Command {
	var f createFlags
	cmd := &cobra.Command{
		Use:   "create --email EMAIL",
		Short: "Create a user",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCreate(cmd.Context(), e, cmd, f)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.email, "email", "", "email address of the new user (required)")
	fl.BoolVar(&f.passwordStdin, "password-stdin", false, "read the password from stdin ($GROBASE_USER_PASSWORD also works)")
	fl.StringVar(&f.role, "role", "", "role claim stored with the user")
	fl.BoolVar(&f.confirm, "confirm", false, "mark the email address as confirmed")
	fl.StringVar(&f.metadata, "metadata", "", "user metadata as a JSON object")
	return cmd
}

func runCreate(ctx context.Context, e *env.Env, cmd *cobra.Command, f createFlags) error {
	if f.email == "" {
		return exitcode.Usagef("--email is required")
	}
	pw, err := readPassword(e, f.passwordStdin)
	if err != nil {
		return err
	}
	md, err := decodeMetadata(f.metadata)
	if err != nil {
		return err
	}
	body := map[string]any{"email": f.email, "email_confirm": f.confirm}
	if pw != "" {
		body["password"] = pw
	}
	if cmd.Flags().Changed("role") {
		body["role"] = f.role
	}
	if md != nil {
		body["user_metadata"] = md
	}
	cl, err := serviceClient(e)
	if err != nil {
		return err
	}
	r := adminReq("POST", adminUsersPath)
	r.JSON = body
	var u map[string]any
	if err := cl.JSON(ctx, r, &u); err != nil {
		return err
	}
	return e.Printer().Print(u)
}

func newInviteCmd(e *env.Env) *cobra.Command {
	var email string
	cmd := &cobra.Command{
		Use:   "invite --email EMAIL",
		Short: "Send an invitation email",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInvite(cmd.Context(), e, email)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "email address to invite (required)")
	return cmd
}

func runInvite(ctx context.Context, e *env.Env, email string) error {
	if email == "" {
		return exitcode.Usagef("--email is required")
	}
	cl, err := serviceClient(e)
	if err != nil {
		return err
	}
	r := adminReq("POST", invitePath)
	r.JSON = map[string]any{"email": email}
	var u map[string]any
	if err := cl.JSON(ctx, r, &u); err != nil {
		return err
	}
	return e.Printer().Print(u)
}

type updateFlags struct {
	email, role, metadata string
	passwordStdin         bool
}

func newUpdateCmd(e *env.Env) *cobra.Command {
	var f updateFlags
	cmd := &cobra.Command{
		Use:   "update <id|email>",
		Short: "Change the email, role, password or metadata of a user",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.Context(), e, cmd, args[0], f)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.email, "email", "", "new email address")
	fl.StringVar(&f.role, "role", "", "new role claim")
	fl.BoolVar(&f.passwordStdin, "password-stdin", false, "read the new password from stdin ($GROBASE_USER_PASSWORD also works)")
	fl.StringVar(&f.metadata, "metadata", "", "new user metadata as a JSON object")
	return cmd
}

func runUpdate(ctx context.Context, e *env.Env, cmd *cobra.Command, id string, f updateFlags) error {
	pw, err := readPassword(e, f.passwordStdin)
	if err != nil {
		return err
	}
	md, err := decodeMetadata(f.metadata)
	if err != nil {
		return err
	}
	body := map[string]any{}
	if cmd.Flags().Changed("email") {
		body["email"] = f.email
	}
	if cmd.Flags().Changed("role") {
		body["role"] = f.role
	}
	if pw != "" {
		body["password"] = pw
	}
	if md != nil {
		body["user_metadata"] = md
	}
	if len(body) == 0 {
		return exitcode.Usagef("nothing to update: pass at least one of --email, --role, --password-stdin or --metadata")
	}
	cl, err := serviceClient(e)
	if err != nil {
		return err
	}
	if id, err = userID(ctx, cl, id); err != nil {
		return err
	}
	r := adminReq("PUT", adminUsersPath+"/"+httpx.Seg(id))
	r.JSON = body
	var u map[string]any
	if err := cl.JSON(ctx, r, &u); err != nil {
		return err
	}
	return e.Printer().Print(u)
}
