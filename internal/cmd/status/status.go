// Package status implements the `grobase status` command.
package status

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

// New returns the `status` command.
func New(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Probe every endpoint of the current context and report its state",
		Long: "GET each endpoint of the current context, concurrently, and report whether it answers.\n" +
			"State is ok (the route answered), down (5xx or no connection), or what a 404 says:\n" +
			"not deployed (the gateway has no route), switched off (the service behind the route\n" +
			"has no handler) or not enabled (a bare 404 that names neither).\n" +
			"Exit code is 1 only when the context URL itself is unreachable; the probes send\n" +
			"no credential, so status works before any `grobase login`.",
		Args: exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), e)
		},
	}
}

func run(ctx context.Context, e *env.Env) error {
	c, err := e.Current()
	if err != nil {
		return err
	}
	// withAuth is false on purpose: it keeps the credential store (and its
	// passphrase prompt) out of a command that must run on a fresh machine.
	cl, err := e.HTTPClient(c, false)
	if err != nil {
		return err
	}
	results := probeAll(ctx, cl, endpoints())
	if err := e.Printer().Print(rows(results), "endpoint", "status", "latency_ms", "state"); err != nil {
		return err
	}
	warnUnreachable(e, results)
	if first := transportError(results); first != nil {
		return fmt.Errorf("status: %s is unreachable: %w", c.URL, first)
	}
	return nil
}

// warnUnreachable puts the cause of every failed probe on stderr: the table
// columns cannot carry it, and a bare "down" tells the operator nothing.
func warnUnreachable(e *env.Env, results []result) {
	for _, r := range results {
		if r.err != nil {
			e.Warnf("probe %s: %v", r.endpoint, r.err)
		}
	}
}
