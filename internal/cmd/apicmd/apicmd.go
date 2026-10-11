// Package apicmd implements the `grobase api` command.
package apicmd

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `api` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "api", Short: "not implemented yet", Hidden: true}
}
