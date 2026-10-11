package cli

import (
	"context"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// trustOnFirstUse probes the server certificate. A certificate that already
// verifies needs no pin (returns ""); otherwise the fingerprint is shown and
// pinned only after the user confirms it (or passes --yes).
func trustOnFirstUse(ctx context.Context, e *env.Env, rawURL string, yes bool) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return "", nil // plain http has no certificate to pin
	}
	host := u.Host
	if u.Port() == "" {
		host += ":443"
	}
	info, err := httpx.Probe(ctx, host)
	if err != nil {
		return "", err
	}
	if info.Verified {
		return "", nil
	}
	_, _ = fmt.Fprintf(e.Stderr, "The server certificate is not trusted by the system roots.\n"+
		"  subject:     %s\n  issuer:      %s\n  expires:     %s\n  fingerprint: sha256:%s\n",
		info.Subject, info.Issuer, info.NotAfter.Format("2006-01-02"), info.SHA256)
	if yes {
		return info.SHA256, nil
	}
	if !e.Interactive() {
		return "", exitcode.Usagef("untrusted certificate: verify the fingerprint out of band, then pass --pin-sha256 %s (or --yes)", info.SHA256)
	}
	ok, err := e.Confirm("Trust and pin this certificate?")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("certificate not trusted")
	}
	return info.SHA256, nil
}

func newContextPin(e *env.Env) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use: "pin [NAME]", Short: "Re-pin the server certificate of a context (after a rotation)",
		Args: exitcode.Args(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			cf, err := e.ConfigStore().Load()
			if err != nil {
				return err
			}
			name := e.Flags.Context
			if len(args) == 1 {
				name = args[0]
			}
			c, err := cf.Get(name)
			if err != nil {
				return err
			}
			pin, err := trustOnFirstUse(cmd.Context(), e, c.URL, yes)
			if err != nil {
				return err
			}
			c.PinSHA256, c.Insecure = pin, false
			if err := cf.Add(c); err != nil {
				return err
			}
			return e.ConfigStore().Save(cf)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "accept the presented fingerprint without asking")
	return cmd
}
