// Package status implements the `grobase status` command.
package status

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `status` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "status", Short: "not implemented yet", Hidden: true}
}
