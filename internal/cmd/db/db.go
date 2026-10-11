// Package db implements the `grobase db` command.
package db

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `db` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "db", Short: "not implemented yet", Hidden: true}
}
