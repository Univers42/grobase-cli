// Package cli assembles the grobase command tree from constructors.
package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/cmd/apicmd"
	"github.com/Univers42/grobase-cli/internal/cmd/apps"
	"github.com/Univers42/grobase-cli/internal/cmd/db"
	"github.com/Univers42/grobase-cli/internal/cmd/doctor"
	"github.com/Univers42/grobase-cli/internal/cmd/functions"
	"github.com/Univers42/grobase-cli/internal/cmd/keys"
	"github.com/Univers42/grobase-cli/internal/cmd/orgs"
	"github.com/Univers42/grobase-cli/internal/cmd/query"
	"github.com/Univers42/grobase-cli/internal/cmd/realtime"
	"github.com/Univers42/grobase-cli/internal/cmd/status"
	"github.com/Univers42/grobase-cli/internal/cmd/storage"
	"github.com/Univers42/grobase-cli/internal/cmd/tenants"
	"github.com/Univers42/grobase-cli/internal/cmd/users"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

// BuildInfo is stamped at link time and shown by `grobase version`.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// NewRoot builds the whole command tree over deps.
func NewRoot(deps env.Deps, info BuildInfo) (*cobra.Command, *env.Env) {
	flags := &env.Flags{}
	deps.Version = info.Version
	e := env.New(deps, flags)
	root := &cobra.Command{
		Use:   "grobase",
		Short: "Remote controller for a self-hosted grobase platform",
		Long: "grobase manages a grobase deployment from the command line: contexts, credentials, users,\n" +
			"API keys, apps, database mounts, queries, storage, realtime and functions.\n\n" +
			"Exit codes: 0 ok, 1 error, 2 usage, 3 authentication/authorization, 4 not found or not enabled.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetIn(deps.Stdin)
	root.SetOut(deps.Stdout)
	root.SetErr(deps.Stderr)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &exitcode.UsageError{Msg: err.Error()} })
	bindFlags(root, flags)
	root.AddCommand(
		newContextCmd(e), newLoginCmd(e), newLogoutCmd(e), newWhoamiCmd(e), newTokenCmd(e),
		newVersionCmd(e, info), newDocsCmd(root),
		users.New(e), keys.New(e), apps.New(e), tenants.New(e), db.New(e), orgs.New(e), query.New(e),
		apicmd.New(e), storage.New(e), realtime.New(e), functions.New(e), status.New(e), doctor.New(e),
	)
	return root, e
}

func bindFlags(root *cobra.Command, f *env.Flags) {
	p := root.PersistentFlags()
	p.StringVar(&f.Context, "context", "", "context to use (default: the current context)")
	p.StringVarP(&f.Output, "output", "o", "table", "output format: table, json or yaml")
	p.StringVar(&f.JQ, "jq", "", "filter the result with a jq expression (prints JSON lines)")
	p.BoolVar(&f.Debug, "debug", false, "trace HTTP requests to stderr (credentials are redacted)")
	p.DurationVar(&f.Timeout, "timeout", 30*time.Second, "time to wait for response headers")
	p.StringVar(&f.CredentialStore, "credential-store", "auto", "credential backend: auto, keyring or file")
}

// ExitCode classifies err, treating cobra's own unknown-command/flag errors as usage errors.
// Ponytail: the cobra errors are recognised by message prefix; a future cobra that
// rewords them would degrade to exit code 1, never to a wrong success.
func ExitCode(err error) int {
	if err != nil && (strings.HasPrefix(err.Error(), "unknown command") || strings.HasPrefix(err.Error(), "unknown flag")) {
		return exitcode.Usage
	}
	return exitcode.Of(err)
}
