package host

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/exitcode"
)

var sinceDuration = regexp.MustCompile(`^[0-9]{1,6}[smh]$`)

func newLogsCmd(o *opts) *cobra.Command {
	var tail int
	var since string
	cmd := &cobra.Command{
		Use:   "logs <service>",
		Short: "Print a container's recent logs, credentials redacted",
		Long: "Prints the last --tail lines of one container (a compose service such as waf, or\n" +
			"its container name), its stdout and stderr together. Each line goes through the\n" +
			"same redaction as --debug: a JSON log line has every credential-named field\n" +
			"masked, any line its JWTs, keys and URL passwords. Caveat: a secret in a shape\n" +
			"and under a name neither layer knows still prints; treat the output as sensitive.",
		Args: exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error { return o.logs(cmd.Context(), args[0], tail, since) },
	}
	cmd.Flags().IntVar(&tail, "tail", 200, "number of lines from the end")
	cmd.Flags().StringVar(&since, "since", "", "only lines newer than this duration (30s, 10m, 2h)")
	return cmd
}

// logs prints the last tail lines of one service, since a duration when set.
func (o *opts) logs(ctx context.Context, service string, tail int, since string) error {
	if tail < 1 || tail > 100000 {
		return exitcode.Usagef("--tail must be between 1 and 100000")
	}
	if since != "" && !sinceDuration.MatchString(since) {
		return exitcode.Usagef("--since takes a duration such as 30s, 10m or 2h")
	}
	dest, err := o.target()
	if err != nil {
		return err
	}
	cs, err := o.containers(ctx, dest)
	if err != nil {
		return err
	}
	names, err := resolve(cs, []string{service})
	if err != nil {
		return err
	}
	words := []string{"docker", "logs", "--tail", strconv.Itoa(tail)}
	if since != "" {
		words = append(words, "--since", since)
	}
	out, err := o.exec(ctx, dest, append(words, names[0], "2>&1")...)
	if err != nil {
		return err
	}
	return o.printRedacted(out)
}

// printRedacted prints out line by line, so each JSON log line is redacted as one document.
func (o *opts) printRedacted(out []byte) error {
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for sc.Scan() {
		if _, err := fmt.Fprintln(o.e.Stdout, o.e.Redact.String(sc.Text())); err != nil {
			return err
		}
	}
	return sc.Err()
}

func newRestartCmd(o *opts) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "restart <service>...",
		Short: "Restart containers of the stack (asks first unless --yes)",
		Long: "Restarts each named container (a compose service or a container name) with\n" +
			"docker restart: the same container and configuration, no rebuild or recreate.\n" +
			"Its open connections drop. Every name is checked against the stack first.",
		Args: exitcode.Args(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			dest, err := o.target()
			if err != nil {
				return err
			}
			cs, err := o.containers(cmd.Context(), dest)
			if err != nil {
				return err
			}
			names, err := resolve(cs, args)
			if err != nil {
				return err
			}
			if err := o.e.ConfirmDestructive(yes, "restart "+strings.Join(names, " ")+" on "+dest); err != nil {
				return err
			}
			if _, err := o.exec(cmd.Context(), dest, append([]string{"docker", "restart"}, names...)...); err != nil {
				return err
			}
			_, err = fmt.Fprintf(o.e.Stdout, "restarted %s on %s\n", strings.Join(names, ", "), dest)
			return err
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "restart without asking")
	return cmd
}
