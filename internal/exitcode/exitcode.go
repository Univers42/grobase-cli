// Package exitcode maps errors to the documented process exit codes.
package exitcode

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/auth"
	"github.com/Univers42/grobase-cli/internal/httpx"
	"github.com/Univers42/grobase-cli/internal/secret"
)

// Exit codes.
const (
	OK       = 0
	Failure  = 1
	Usage    = 2
	AuthFail = 3
	NotFound = 4
)

// UsageError marks a user mistake in flags or arguments.
type UsageError struct{ Msg string }

// Error implements error.
func (e *UsageError) Error() string { return e.Msg }

// Of returns the exit code for err.
func Of(err error) int {
	var ue *UsageError
	var ae *httpx.APIError
	switch {
	case err == nil:
		return OK
	case errors.As(err, &ue):
		return Usage
	case errors.Is(err, auth.ErrNoCredential), errors.Is(err, secret.ErrBadPassphrase):
		return AuthFail
	case errors.As(err, &ae) && ae.Unauthorized():
		return AuthFail
	case errors.As(err, &ae) && ae.NotFound(), errors.Is(err, fs.ErrNotExist):
		return NotFound
	}
	return Failure
}

// Args wraps a cobra positional-argument validator so its failures exit with Usage.
func Args(v cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := v(cmd, args); err != nil {
			return &UsageError{Msg: err.Error()}
		}
		return nil
	}
}

// Usagef builds a UsageError.
func Usagef(format string, a ...any) error { return &UsageError{Msg: fmt.Sprintf(format, a...)} }
