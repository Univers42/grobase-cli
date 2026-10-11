// Package functions implements the `grobase functions` command.
package functions

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `functions` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "functions", Short: "not implemented yet", Hidden: true}
}
