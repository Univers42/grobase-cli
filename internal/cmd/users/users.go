// Package users implements the `grobase users` command.
package users

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
)

// New returns the `users` command.
func New(e *env.Env) *cobra.Command {
	_ = e
	return &cobra.Command{Use: "users", Short: "not implemented yet", Hidden: true}
}
