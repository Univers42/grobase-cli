package host

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

// psOut is `docker ps -a --format '{{json .}}'` as the baas VM answers it, trimmed.
const psOut = `{"Names":"mini-baas-waf","State":"running","Status":"Up 2 hours (healthy)","Image":"ghcr.io/univers42/grobase-waf:latest","Labels":"com.docker.compose.project=mini-baas,com.docker.compose.service=waf"}
{"Names":"mini-baas-kong","State":"running","Status":"Up 2 hours (unhealthy)","Image":"kong:3.9","Labels":"com.docker.compose.service=kong,com.docker.compose.project=mini-baas"}
{"Names":"mini-baas-seed","State":"exited","Status":"Exited (0) 2 hours ago","Image":"seed","Labels":"com.docker.compose.service=seed"}
{"Names":"mini-baas-worker","State":"restarting","Status":"Restarting (1) 3 seconds ago","Image":"worker","Labels":"com.docker.compose.service=worker"}
{"Names":"mini-baas-mig","State":"exited","Status":"Exited (2) 1 hour ago","Image":"mig","Labels":"com.docker.compose.service=mig"}
`

// fakeHost records every command line and answers docker ps with ps, logs with logs.
type fakeHost struct {
	ps, logs string
	targets  []string
	commands []string
}

func (f *fakeHost) run(_ context.Context, target, command string) ([]byte, error) {
	f.targets = append(f.targets, target)
	f.commands = append(f.commands, command)
	switch {
	case strings.HasPrefix(command, "docker ps "):
		return []byte(f.ps), nil
	case strings.HasPrefix(command, "docker logs "):
		return []byte(f.logs), nil
	}
	return nil, nil
}

func runHost(t *testing.T, f *fakeHost, stdin string, args ...string) (*testenv.Harness, error) {
	t.Helper()
	h := testenv.New(t, nil, map[string]string{secret.KindAnon: "anon"}, stdin)
	return h, h.Run(newWith(h.Env, f.run), args...)
}

func TestPsCommandLineIsFixed(t *testing.T) {
	f := &fakeHost{ps: psOut}
	h, err := runHost(t, f, "", "ps", "--ssh", "b2b")
	if err != nil {
		t.Fatal(err)
	}
	want := `docker ps -a --no-trunc --filter label=com.docker.compose.project=mini-baas --format '{{json .}}'`
	if f.commands[0] != want || f.targets[0] != "b2b" {
		t.Fatalf("ran %q on %q", f.commands[0], f.targets[0])
	}
	if !strings.Contains(h.Out.String(), `"health": "unhealthy"`) && !strings.Contains(h.Out.String(), `"health":"unhealthy"`) {
		t.Errorf("health not parsed: %s", h.Out)
	}
}

func TestHealthNamesEveryProblemAndExits1(t *testing.T) {
	_, err := runHost(t, &fakeHost{ps: psOut}, "", "health", "--ssh", "b2b")
	if exitcode.Of(err) != 1 {
		t.Fatalf("exit %d: %v", exitcode.Of(err), err)
	}
	for _, want := range []string{"3 of 5", "mini-baas-kong: unhealthy", "mini-baas-worker: restarting", "mini-baas-mig: exited with code 2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "seed") || strings.Contains(err.Error(), "waf") {
		t.Errorf("a healthy or cleanly exited container was blamed: %v", err)
	}
	healthy := strings.SplitN(psOut, "\n", 2)[0] + "\n"
	if _, err := runHost(t, &fakeHost{ps: healthy}, "", "health", "--ssh", "b2b"); err != nil {
		t.Errorf("a healthy stack failed: %v", err)
	}
}

func TestRefusedInputNeverReachesTheHost(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"ssh option injection", []string{"ps", "--ssh", "-oProxyCommand=touch /tmp/x"}, exitcode.Usage},
		{"ssh with a space", []string{"ps", "--ssh", "b2b; rm -rf /"}, exitcode.Usage},
		{"project with shell", []string{"ps", "--ssh", "b2b", "--project", "x$(id)"}, exitcode.Usage},
		{"no destination", []string{"ps"}, exitcode.Usage},
		{"since not a duration", []string{"logs", "waf", "--ssh", "b2b", "--since", "1h;id"}, exitcode.Usage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeHost{ps: psOut}
			_, err := runHost(t, f, "", tc.args...)
			if exitcode.Of(err) != tc.want || len(f.commands) != 0 {
				t.Fatalf("exit %d (%v), %d command(s) sent", exitcode.Of(err), err, len(f.commands))
			}
		})
	}
	f := &fakeHost{ps: psOut}
	_, err := runHost(t, f, "", "logs", "--ssh", "b2b", "waf;id")
	if exitcode.Of(err) != exitcode.Usage || len(f.commands) != 1 {
		t.Fatalf("a bad service name must stop after the listing: exit %d, %v", exitcode.Of(err), f.commands)
	}
}

func TestLogsResolvesTheServiceAndRedactsEachLine(t *testing.T) {
	logs := `{"level":"info","msg":"login","client_secret":"opaque-client-value"}` + "\n" +
		"connect postgres://app:pw-opaque@db:5432/app\nplain line\n"
	f := &fakeHost{ps: psOut, logs: logs}
	h, err := runHost(t, f, "", "logs", "--ssh", "b2b", "--tail", "50", "--since", "10m", "waf")
	if err != nil {
		t.Fatal(err)
	}
	if f.commands[1] != "docker logs --tail 50 --since 10m mini-baas-waf 2>&1" {
		t.Errorf("ran %q", f.commands[1])
	}
	out := h.Out.String()
	for _, leak := range []string{"opaque-client-value", "pw-opaque"} {
		if strings.Contains(out, leak) {
			t.Errorf("logs leaked %q:\n%s", leak, out)
		}
	}
	if !strings.Contains(out, "plain line") || !strings.Contains(out, `"msg":"login"`) {
		t.Errorf("logs lost content:\n%s", out)
	}
}

func TestRestartAsksThenRunsOneFixedLine(t *testing.T) {
	f := &fakeHost{ps: psOut}
	_, err := runHost(t, f, "", "restart", "--ssh", "b2b", "kong", "mini-baas-waf")
	if exitcode.Of(err) != exitcode.Usage || len(f.commands) != 1 {
		t.Fatalf("without --yes and no terminal it must refuse after the listing: exit %d, %v", exitcode.Of(err), f.commands)
	}
	f = &fakeHost{ps: psOut}
	h, err := runHost(t, f, "", "restart", "--ssh", "b2b", "--yes", "kong", "mini-baas-waf")
	if err != nil {
		t.Fatal(err)
	}
	if f.commands[1] != "docker restart mini-baas-kong mini-baas-waf" || !strings.Contains(h.Out.String(), "restarted") {
		t.Errorf("ran %q, printed %q", f.commands[1], h.Out)
	}
	_, err = runHost(t, &fakeHost{ps: psOut}, "", "restart", "--ssh", "b2b", "--yes", "redis")
	var nf *exitcode.NotFoundError
	if !errors.As(err, &nf) || !strings.Contains(err.Error(), "services: ") {
		t.Errorf("an unknown service must be not-found and list the stack: %v", err)
	}
}

func TestParsePSHealthAndExitCode(t *testing.T) {
	cs, err := parsePS([]byte(psOut))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]container{}
	for _, c := range cs {
		got[c.Service] = c
	}
	if got["waf"].Health != "healthy" || got["kong"].Health != "unhealthy" || got["mig"].ExitCode != 2 || got["seed"].ExitCode != 0 {
		t.Errorf("parsed %+v", got)
	}
	if _, err := parsePS([]byte("Cannot connect to the Docker daemon\n")); err == nil {
		t.Error("a non-JSON answer must be an error, not an empty stack")
	}
}
