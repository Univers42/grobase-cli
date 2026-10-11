// Package tenants implements the `grobase tenants` command.
package tenants

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `tenants` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "tenants", Short: "not implemented yet", Hidden: true}
}
