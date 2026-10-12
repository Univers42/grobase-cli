// Package host implements `grobase host`: the machine that runs grobase's containers,
// reached over the system ssh client. grobase has no API for its own containers, and
// one would be a remote shell behind the gateway; ssh is the access the operator
// already has, with their keys, agent and ~/.ssh/config.
//
// Every remote command line is built here from fixed words and values checked against
// a closed character set, so nothing a flag carries is ever read by the remote shell.
package host

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

// remoteTimeout bounds one ssh command: a restart waits up to docker's 10 s stop
// timeout per container, so a stack-wide restart fits; a hung host does not stall us.
// Caveat: a restart of many slow-stopping containers can overrun it; the restart goes
// on on the host and `grobase host ps` shows where it got to.
const remoteTimeout = 3 * time.Minute

// remote runs one command line on target and returns its stdout.
type remote func(ctx context.Context, target, command string) ([]byte, error)

// opts is what every host subcommand shares.
type opts struct {
	e       *env.Env
	run     remote
	ssh     string
	project string
}

var (
	projectName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)
	objectName  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
)

// New returns the `host` command.
func New(e *env.Env) *cobra.Command { return newWith(e, sshRemote) }

func newWith(e *env.Env, run remote) *cobra.Command {
	o := &opts{e: e, run: run}
	c := &cobra.Command{
		Use:   "host",
		Short: "Inspect and restart the containers of the machine running grobase (over ssh)",
		Long: "Runs docker on the machine that hosts grobase, through the system ssh client:\n" +
			"the destination is --ssh, else the context's ssh (grobase context add --ssh).\n" +
			"The ssh login must be allowed to run docker (the docker group). Only fixed\n" +
			"command lines are sent, every name checked first.",
		Args: exitcode.Args(cobra.NoArgs),
	}
	p := c.PersistentFlags()
	p.StringVar(&o.ssh, "ssh", "", "ssh destination (default: the context's ssh)")
	p.StringVar(&o.project, "project", "mini-baas", "docker compose project of the grobase stack")
	c.AddCommand(newPsCmd(o), newHealthCmd(o), newLogsCmd(o), newRestartCmd(o))
	return c
}

// target resolves and checks the ssh destination and the compose project.
func (o *opts) target() (string, error) {
	if !projectName.MatchString(o.project) {
		return "", exitcode.Usagef("invalid --project %q: lowercase letters, digits, '_' and '-'", o.project)
	}
	dest := o.ssh
	if dest == "" {
		c, err := o.e.Current()
		if err != nil {
			return "", err
		}
		dest = c.SSH
	}
	if dest == "" {
		return "", exitcode.Usagef("no ssh destination: pass --ssh <alias|user@host> or add one to the context (grobase context add --ssh)")
	}
	if !config.ValidSSHTarget(dest) {
		return "", exitcode.Usagef("invalid ssh destination %q: an alias, host or user@host, not starting with '-'", dest)
	}
	return dest, nil
}

// exec runs words, already checked, as one command line on the host.
func (o *opts) exec(ctx context.Context, dest string, words ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, remoteTimeout)
	defer cancel()
	return o.run(ctx, dest, strings.Join(words, " "))
}

// sshRemote runs command through ssh: BatchMode so a missing key fails instead of
// prompting, and `--` so the destination can never be read as an ssh option.
func sshRemote(ctx context.Context, target, command string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	//nolint:gosec // G204: target passed ValidSSHTarget and command is built from checked words (see package doc)
	cmd := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "--", target, command)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		msg := strings.TrimSpace(stderr.String())
		if errors.As(err, &exitErr) && msg != "" {
			return stdout.Bytes(), fmt.Errorf("ssh %s: %s", target, lastLines(msg, 5))
		}
		return stdout.Bytes(), fmt.Errorf("ssh %s: %w", target, err)
	}
	return stdout.Bytes(), nil
}

// lastLines keeps the end of a long stderr, where ssh and docker put the cause.
func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

// quoteTemplate single-quotes a fixed docker --format template for the remote shell.
func quoteTemplate(t string) string { return "'" + t + "'" }
