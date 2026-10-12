package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
)

// newContextCmd builds the context command tree.
func newContextCmd(e *env.Env) *cobra.Command {
	c := &cobra.Command{Use: "context", Short: "Manage deployments (contexts)"}
	c.AddCommand(newContextAdd(e), newContextList(e), newContextUse(e), newContextRemove(e),
		newContextShow(e), newContextPin(e), newContextEnv(e))
	return c
}

type addFlags struct {
	urls          []string
	caFile, pin   string
	ssh           string
	insecure, yes bool
}

// newContextAdd builds the context add command.
func newContextAdd(e *env.Env) *cobra.Command {
	var f addFlags
	cmd := &cobra.Command{
		Use:   "add NAME --url URL",
		Short: "Add (or replace) a context; pins the server certificate on first use when it is untrusted",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContextAdd(cmd, e, args[0], f)
		},
	}
	cmd.Flags().StringArrayVar(&f.urls, "url", nil, "edge URL, e.g. https://host:8443 (repeatable; all URLs must present the same certificate)")
	cmd.Flags().StringVar(&f.caFile, "ca-file", "", "PEM CA bundle to trust in addition to the system roots")
	cmd.Flags().StringVar(&f.pin, "pin-sha256", "", "pin the server leaf certificate (hex SHA-256 of its DER)")
	cmd.Flags().BoolVar(&f.insecure, "insecure-skip-verify", false, "DANGEROUS: do not verify the server certificate at all")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "accept the presented certificate fingerprint without asking")
	cmd.Flags().StringVar(&f.ssh, "ssh", "", "ssh destination of the machine running grobase, for `grobase host` (alias or user@host)")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

// dedupURLs drops duplicate URLs while keeping the first occurrence order.
func dedupURLs(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// runContextAdd saves a context with the first URL as primary and the rest as alternates.
func runContextAdd(cmd *cobra.Command, e *env.Env, name string, f addFlags) error {
	urls := dedupURLs(f.urls)
	if len(urls) == 0 {
		return &exitcode.UsageError{Msg: "missing required --url"}
	}
	c := config.Context{Name: name, URL: urls[0], CAFile: f.caFile, PinSHA256: f.pin, Insecure: f.insecure, SSH: f.ssh}
	if len(urls) > 1 {
		c.Alternates = urls[1:]
	}
	if err := c.Validate(); err != nil {
		return &exitcode.UsageError{Msg: err.Error()}
	}
	if f.insecure {
		e.Warnf("--insecure-skip-verify disables certificate verification; anyone on the path can read and alter traffic")
	}
	if c.PinSHA256 == "" && !c.Insecure && c.CAFile == "" {
		pin, err := trustOnFirstUse(cmd.Context(), e, c.URL, f.yes)
		if err != nil {
			return err
		}
		c.PinSHA256 = pin
	}
	cf, err := e.ConfigStore().Load()
	if err != nil {
		return err
	}
	if err := cf.Add(c); err != nil {
		return err
	}
	if err := e.ConfigStore().Save(cf); err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.Stdout, "context %q saved (current: %s)\n", name, cf.Current)
	return err
}

// newContextList builds the context list command.
func newContextList(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List contexts", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			cf, err := e.ConfigStore().Load()
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(cf.Contexts))
			for _, n := range cf.Names() {
				c := cf.Contexts[n]
				rows = append(rows, map[string]any{"name": n, "current": n == cf.Current, "url": c.URL, "tls": tlsMode(c), "ssh": c.SSH})
			}
			return e.Printer().Print(rows, "name", "current", "url", "tls", "ssh")
		},
	}
}

// tlsMode names the trust mode of a context.
func tlsMode(c config.Context) string {
	switch {
	case c.Insecure:
		return "insecure"
	case c.PinSHA256 != "":
		return "pinned"
	case c.CAFile != "":
		return "ca-file"
	}
	return "system"
}

// newContextUse builds the context use command.
func newContextUse(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use: "use NAME", Short: "Select the current context", Args: exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			cf, err := e.ConfigStore().Load()
			if err != nil {
				return err
			}
			if err := cf.Use(args[0]); err != nil {
				return err
			}
			return e.ConfigStore().Save(cf)
		},
	}
}

// newContextRemove builds the context remove command.
func newContextRemove(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use: "remove NAME", Short: "Remove a context and its stored credentials", Args: exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			cf, err := e.ConfigStore().Load()
			if err != nil {
				return err
			}
			if err := cf.Remove(args[0]); err != nil {
				return err
			}
			if err := e.ConfigStore().Save(cf); err != nil {
				return err
			}
			return deleteCredentials(e, args[0])
		},
	}
}

// deleteCredentials removes every stored credential of a context.
func deleteCredentials(e *env.Env, ctxName string) error {
	store, err := e.Secrets()
	if err != nil {
		return err
	}
	for _, kind := range []string{secret.KindAnon, secret.KindServiceRole, secret.KindAPIKey, secret.KindSession} {
		if err := store.Delete(secret.Name(ctxName, kind)); err != nil {
			return err
		}
	}
	return nil
}

// newContextShow builds the context show command.
func newContextShow(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use: "show [NAME]", Short: "Show one context (default: current)", Args: exitcode.Args(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
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
			m := map[string]any{
				"name": c.Name, "url": c.URL, "tls": tlsMode(c), "ca_file": c.CAFile, "pin_sha256": c.PinSHA256,
			}
			if len(c.Alternates) > 0 {
				m["alternates"] = c.Alternates
			}
			return e.Printer().Print(m)
		},
	}
}

// shellQuote wraps a value in single quotes with POSIX escaping.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// newContextEnv builds the context env command.
func newContextEnv(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use: "env [NAME]", Short: "Print shell assignments for the chosen URL of a context", Args: exitcode.Args(cobra.MaximumNArgs(1)),
		RunE: func(_ *cobra.Command, args []string) error {
			name := e.Flags.Context
			if len(args) == 1 {
				name = args[0]
			}
			return runContextEnv(e, name)
		},
	}
}

// runContextEnv prints GROBASE_TARGET and GROBASE_CA_FILE for shells and make.
func runContextEnv(e *env.Env, name string) error {
	cf, err := e.ConfigStore().Load()
	if err != nil {
		return err
	}
	c, err := cf.Get(name)
	if err != nil {
		return err
	}
	target, err := e.SelectedURL(c)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(e.Stdout, "GROBASE_TARGET=%s\n", shellQuote(target)); err != nil {
		return err
	}
	if c.CAFile == "" {
		return nil
	}
	abs, err := filepath.Abs(c.CAFile)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.Stdout, "GROBASE_CA_FILE=%s\n", shellQuote(abs))
	return err
}
