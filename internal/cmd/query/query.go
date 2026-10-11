// Package query implements the `grobase query` command.
package query

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `query` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "query", Short: "not implemented yet", Hidden: true}
}
