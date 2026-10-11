// Package storage implements the `grobase storage` command.
package storage

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `storage` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "storage", Short: "not implemented yet", Hidden: true}
}
