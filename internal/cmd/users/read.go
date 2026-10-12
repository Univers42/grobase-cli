package users

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// userPage is one page of the GoTrue admin user list.
type userPage struct {
	Users []map[string]any `json:"users"`
	Total int              `json:"total"`
}

func newListCmd(e *env.Env) *cobra.Command {
	var page, perPage int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runList(cmd.Context(), e, page, perPage)
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "page number, starting at 1")
	cmd.Flags().IntVar(&perPage, "per-page", 50, "users per page")
	return cmd
}

func runList(ctx context.Context, e *env.Env, page, perPage int) error {
	if page < 1 || perPage < 1 {
		return exitcode.Usagef("--page and --per-page start at 1")
	}
	cl, err := serviceClient(e)
	if err != nil {
		return err
	}
	r := adminReq(http.MethodGet, adminUsersPath)
	r.Query = url.Values{"page": {strconv.Itoa(page)}, "per_page": {strconv.Itoa(perPage)}}
	var out userPage
	if err := cl.JSON(ctx, r, &out); err != nil {
		return err
	}
	// A page shorter than the total is the only hint GoTrue gives about the rest.
	if out.Total > len(out.Users) {
		e.Warnf("%d users in total, this page shows %d (see --page and --per-page)", out.Total, len(out.Users))
	}
	return e.Printer().Print(usersOrEmpty(out.Users), "id", "email", "role", "created_at", "banned_until")
}

// usersOrEmpty keeps a missing list a JSON array, so a table still gets its header row.
func usersOrEmpty(users []map[string]any) []map[string]any {
	if users == nil {
		return []map[string]any{}
	}
	return users
}

func newGetCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id|email>",
		Short: "Show one user by id or by email",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd.Context(), e, args[0])
		},
	}
}

func runGet(ctx context.Context, e *env.Env, idOrEmail string) error {
	cl, err := serviceClient(e)
	if err != nil {
		return err
	}
	u, err := fetchUser(ctx, cl, idOrEmail)
	if err != nil {
		return err
	}
	return e.Printer().Print(u)
}

// fetchUser resolves an id directly; an argument with an "@" is treated as an email.
func fetchUser(ctx context.Context, cl *httpx.Client, idOrEmail string) (map[string]any, error) {
	if strings.Contains(idOrEmail, "@") {
		return userByEmail(ctx, cl, idOrEmail)
	}
	return userByID(ctx, cl, idOrEmail)
}

// userID turns an id-or-email argument into the id the admin write paths need.
// An email sent as-is lands in the URL path, where the b2b WAF answered 403
// rather than GoTrue's 404 (2026-10-12), so the user saw a firewall refusal for
// a typo-free command.
func userID(ctx context.Context, cl *httpx.Client, idOrEmail string) (string, error) {
	if !strings.Contains(idOrEmail, "@") {
		return idOrEmail, nil
	}
	u, err := userByEmail(ctx, cl, idOrEmail)
	if err != nil {
		return "", err
	}
	id, _ := u["id"].(string)
	if id == "" {
		return "", notFound("user " + idOrEmail + " carries no id")
	}
	return id, nil
}

func userByID(ctx context.Context, cl *httpx.Client, id string) (map[string]any, error) {
	var u map[string]any
	err := cl.JSON(ctx, adminReq(http.MethodGet, adminUsersPath+"/"+httpx.Seg(id)), &u)
	return u, err
}

// userByEmail scans the listed users for an exact email match.
// Ponytail: only the first 1000 users are looked at; a bigger deployment needs paging here.
func userByEmail(ctx context.Context, cl *httpx.Client, email string) (map[string]any, error) {
	r := adminReq(http.MethodGet, adminUsersPath)
	r.Query = url.Values{"per_page": {"1000"}}
	var out userPage
	if err := cl.JSON(ctx, r, &out); err != nil {
		return nil, err
	}
	for _, u := range out.Users {
		if u["email"] == email {
			return u, nil
		}
	}
	return nil, notFound("no user with email " + email)
}

// notFound builds the same error shape the client returns for a 404, so the exit code
// (4) and the wording stay in one place.
func notFound(msg string) error {
	return &httpx.APIError{Method: http.MethodGet, Path: adminUsersPath, Status: http.StatusNotFound, Message: msg}
}
