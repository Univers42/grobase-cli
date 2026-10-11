// Package realtime implements the `grobase realtime` command.
package realtime

import (
	_ "github.com/coder/websocket" // replaced by the realtime implementation
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `realtime` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "realtime", Short: "not implemented yet", Hidden: true}
}
