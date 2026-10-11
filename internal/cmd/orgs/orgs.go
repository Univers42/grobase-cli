// Package orgs implements the `grobase orgs` command.
package orgs

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `orgs` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "orgs", Short: "not implemented yet", Hidden: true}
}
