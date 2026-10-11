// Package users implements the `grobase users` command: user administration through
// the GoTrue admin API. Every request authenticates with the stored service-role key,
// because only that key may manage somebody else's account.
package users

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

// GoTrue routes used by this package.
const (
	adminUsersPath = "/auth/v1/admin/users"
	invitePath     = "/auth/v1/invite"
)

// New returns the `users` command.
func New(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "users",
		Short: "Manage the users of a deployment (admin: needs the service-role key)",
		Long: "List, inspect, create, update, ban and delete users, send invitations and revoke sessions.\n\n" +
			"These calls hit the GoTrue admin API and therefore authenticate with the stored service-role\n" +
			"key (`grobase login --service-role`).",
		Args: exitcode.Args(cobra.NoArgs),
	}
	c.AddCommand(
		newListCmd(e), newGetCmd(e), newCreateCmd(e), newInviteCmd(e),
		newUpdateCmd(e), newDeleteCmd(e), newBanCmd(e), newUnbanCmd(e),
		newSessionsCmd(e),
	)
	return c
}
