// Package realtime implements the `grobase realtime` command.
package realtime

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

// wsPath is the realtime socket Kong exposes.
const wsPath = "/realtime/v1/ws"

// New returns the `realtime` command.
func New(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "realtime",
		Short: "Subscribe to and publish on realtime channels over WebSocket",
		Long: "The socket authenticates with the session of the current context: an AUTH\n" +
			"frame carries the same bearer the HTTP calls send, right after the upgrade.\n" +
			"An anon-only context cannot use realtime; run `grobase login` first.",
		Args: exitcode.Args(cobra.NoArgs),
	}
	c.AddCommand(newSubscribeCmd(e), newPublishCmd(e))
	return c
}

// channelArg trims the positional channel name and rejects an empty one, which
// the server would answer with a confusing protocol error.
func channelArg(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", exitcode.Usagef("the channel must not be empty")
	}
	return s, nil
}
