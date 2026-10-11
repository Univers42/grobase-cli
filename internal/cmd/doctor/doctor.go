// Package doctor implements the `grobase doctor` command: an ordered health
// report of the current context, from the config file to the feature flags the
// deployment carries.
package doctor

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

// New returns the `doctor` command.
func New(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the current context: config, connectivity, TLS, clock, credentials, auth and features",
		Long: "Run the checks in order and report each one as PASS, WARN, FAIL or SKIP.\n" +
			"Exit code is 1 as soon as one check fails. Nothing is written: doctor only\n" +
			"reads the configuration, the credential store and the deployment.",
		Args: exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return report(cmd.Context(), e)
		},
	}
}

// report runs every check in order, prints the rows and fails when one of them
// did. The order is the diagnosis order: a missing context explains everything
// that follows, so it is reported first.
func report(ctx context.Context, e *env.Env) error {
	cs := runChecks(ctx, e)
	if err := e.Printer().Print(rowsOf(cs), "check", "result", "detail"); err != nil {
		return err
	}
	if n := summarise(e, cs); n > 0 {
		return fmt.Errorf("doctor: %d check(s) failed", n)
	}
	return nil
}

// runChecks returns the rows in the declared order. Every check shares one run,
// so the whole report is based on a single view of the configuration.
func runChecks(ctx context.Context, e *env.Env) []check {
	r := newRun(ctx, e)
	out := []check{}
	for _, c := range checks() {
		out = append(out, c(r)...)
	}
	return out
}

// checks lists the checks in the order they run and are reported.
func checks() []func(*run) []check {
	return []func(*run) []check{
		checkConfig, checkReach, checkTLS, checkSkew, checkCreds, checkAuth, checkFeatures,
	}
}

// summarise writes the tally on stderr: the piped stdout stays one row per check
// for a machine to read, while the operator still gets a verdict. It returns the
// number of failed checks.
func summarise(e *env.Env, cs []check) int {
	n := map[string]int{}
	for _, c := range cs {
		n[c.result]++
	}
	_, _ = fmt.Fprintf(e.Stderr, "doctor: %d pass, %d warn, %d fail, %d skip\n",
		n[resultPass], n[resultWarn], n[resultFail], n[resultSkip])
	return n[resultFail]
}

// rowsOf turns the checks into printer rows. The fields are unexported on
// purpose, so this is the only place a check reaches the output.
func rowsOf(cs []check) []map[string]any {
	out := make([]map[string]any, 0, len(cs))
	for _, c := range cs {
		out = append(out, map[string]any{"check": c.name, "result": c.result, "detail": c.detail})
	}
	return out
}
