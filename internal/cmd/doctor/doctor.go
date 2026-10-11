// Package doctor implements the `grobase doctor` command.
package doctor

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `doctor` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "doctor", Short: "not implemented yet", Hidden: true}
}
