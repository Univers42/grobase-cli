// Command grobase is a remote controller for a self-hosted grobase platform.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"golang.org/x/term"

	"github.com/Univers42/grobase-cli/internal/cli"
	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/env"
)

// Stamped by -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() { os.Exit(run()) }

func run() int {
	cfgPath, err := config.DefaultPath(os.Getenv, os.UserHomeDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fd := int(os.Stdin.Fd())
	deps := env.Deps{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		Clock: time.Now, Getenv: os.Getenv, ConfigPath: cfgPath,
		Interactive: func() bool { return term.IsTerminal(fd) },
		ReadPassword: func(prompt string) (string, error) {
			fmt.Fprint(os.Stderr, prompt)
			b, err := term.ReadPassword(fd)
			fmt.Fprintln(os.Stderr)
			return string(b), err
		},
	}
	root, e := cli.NewRoot(deps, cli.BuildInfo{Version: version, Commit: commit, Date: date})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", e.Redact.String(err.Error()))
		return cli.ExitCode(err)
	}
	return 0
}
