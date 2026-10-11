// Package keys implements the `grobase keys` command.
package keys

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `keys` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "keys", Short: "not implemented yet", Hidden: true}
}
