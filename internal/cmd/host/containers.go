package host

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/exitcode"
)

// container is one row of `docker ps -a` for the stack.
type container struct {
	Service, Name, State, Health, Status, Image string
	ExitCode                                    int
}

// psLine is the part of docker's `{{json .}}` row this reads.
type psLine struct {
	Names, State, Status, Image, Labels string
}

var (
	healthInStatus = regexp.MustCompile(`\((healthy|unhealthy|health: starting)\)`)
	exitInStatus   = regexp.MustCompile(`^Exited \((-?\d+)\)`)
)

// containers lists every container of the compose project, running or not.
func (o *opts) containers(ctx context.Context, dest string) ([]container, error) {
	out, err := o.exec(ctx, dest, "docker", "ps", "-a", "--no-trunc",
		"--filter", "label=com.docker.compose.project="+o.project, "--format", quoteTemplate("{{json .}}"))
	if err != nil {
		return nil, err
	}
	return parsePS(out)
}

// parsePS reads docker's JSON lines. Health and exit code live only in the Status text.
// Caveat: that text is docker's display string ("Up 2 hours (healthy)", "Exited (1) 5
// minutes ago"); a docker that rewords it reads as no health and exit code 0, which
// under-reports a failure, so health also treats an unknown State as a problem.
func parsePS(out []byte) ([]container, error) {
	var cs []container
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var p psLine
		if err := json.Unmarshal(line, &p); err != nil {
			return nil, fmt.Errorf("docker ps answered a line that is not JSON: %w", err)
		}
		c := container{Name: p.Names, State: p.State, Status: p.Status, Image: p.Image, Service: label(p.Labels, "com.docker.compose.service")}
		if m := healthInStatus.FindStringSubmatch(p.Status); m != nil {
			c.Health = strings.TrimPrefix(m[1], "health: ")
		}
		if m := exitInStatus.FindStringSubmatch(p.Status); m != nil {
			c.ExitCode, _ = strconv.Atoi(m[1])
		}
		cs = append(cs, c)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].Service < cs[j].Service })
	return cs, nil
}

// label reads one key of docker's "k=v,k=v" label string.
func label(labels, key string) string {
	for _, kv := range strings.Split(labels, ",") {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v
		}
	}
	return ""
}

// problem names what is wrong with c, or "" when nothing is: the verdict of
// born2root's b2b-stack-health (unhealthy, crash-looping, exited non-zero).
func problem(c container) string {
	switch {
	case c.Health == "unhealthy":
		return "unhealthy"
	case c.State == "restarting":
		return "restarting (crash loop)"
	case c.State == "exited" && c.ExitCode != 0:
		return fmt.Sprintf("exited with code %d", c.ExitCode)
	case c.State == "dead":
		return "dead"
	case c.State != "running" && c.State != "exited" && c.State != "created" && c.State != "paused":
		return "state " + c.State
	}
	return ""
}

func rows(cs []container) []map[string]any {
	out := make([]map[string]any, 0, len(cs))
	for _, c := range cs {
		health := c.Health
		if health == "" {
			health = "-"
		}
		out = append(out, map[string]any{
			"service": c.Service, "name": c.Name, "state": c.State, "health": health, "status": c.Status, "image": c.Image,
		})
	}
	return out
}

func newPsCmd(o *opts) *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "List the stack's containers with their state and health",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			dest, err := o.target()
			if err != nil {
				return err
			}
			cs, err := o.containers(cmd.Context(), dest)
			if err != nil {
				return err
			}
			return o.e.Printer().Print(rows(cs), "service", "name", "state", "health", "status")
		},
	}
}

func newHealthCmd(o *opts) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Exit 1 naming each container that is unhealthy, crash-looping or exited non-zero",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			dest, err := o.target()
			if err != nil {
				return err
			}
			cs, err := o.containers(cmd.Context(), dest)
			if err != nil {
				return err
			}
			if len(cs) == 0 {
				return fmt.Errorf("no container of project %q on %s: is the stack up, or is --project wrong?", o.project, dest)
			}
			var bad []string
			for _, c := range cs {
				if p := problem(c); p != "" {
					bad = append(bad, c.Name+": "+p)
				}
			}
			if len(bad) > 0 {
				return fmt.Errorf("%d of %d containers need attention: %s", len(bad), len(cs), strings.Join(bad, "; "))
			}
			_, err = fmt.Fprintf(o.e.Stdout, "%d containers of %s: none unhealthy, crash-looping or failed\n", len(cs), o.project)
			return err
		},
	}
}

// resolve maps each argument, a compose service or a container name, to the container
// name docker takes; an unknown one is refused before anything runs.
func resolve(cs []container, args []string) ([]string, error) {
	names := make([]string, 0, len(args))
	for _, a := range args {
		if !objectName.MatchString(a) {
			return nil, exitcode.Usagef("invalid service %q", a)
		}
		found := ""
		for _, c := range cs {
			if c.Service == a || c.Name == a {
				found = c.Name
				break
			}
		}
		if found == "" {
			services := make([]string, 0, len(cs))
			for _, c := range cs {
				services = append(services, c.Service)
			}
			return nil, exitcode.NotFoundf("no service or container %q in the stack (services: %s)", a, strings.Join(services, ", "))
		}
		if !objectName.MatchString(found) {
			return nil, fmt.Errorf("docker named a container %q, which this CLI will not pass to a shell", found)
		}
		names = append(names, found)
	}
	return names, nil
}
