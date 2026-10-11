package status_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/status"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

func TestStatusStates(t *testing.T) {
	h := testenv.New(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("apikey") != "" {
			t.Errorf("%s: status must send no credential", r.URL.Path)
		}
		switch r.URL.Path {
		case "/auth/v1/health":
			w.WriteHeader(200)
		case "/storage/v1/bucket", "/v1/tenants/me":
			w.WriteHeader(401)
		case "/query/v1/engines":
			w.WriteHeader(404)
		default:
			w.WriteHeader(503)
		}
	}), nil, "")
	if err := h.Run(status.New(h.Env)); err != nil {
		t.Fatalf("exit %d: %v", exitcode.Of(err), err)
	}
	out := h.Out.String()
	for _, want := range []string{`"state": "ok"`, `"state": "not enabled"`, `"state": "down"`, `"endpoint": "/auth/v1/health"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in %s", want, out)
		}
	}
}

func TestStatusUnreachableContext(t *testing.T) {
	h := testenv.New(t, http.NotFoundHandler(), nil, "")
	h.Server.Close()
	err := h.Run(status.New(h.Env))
	if exitcode.Of(err) != exitcode.Failure {
		t.Fatalf("exit %d, want 1 (err=%v)", exitcode.Of(err), err)
	}
}

func TestStatusNoContext(t *testing.T) {
	h := testenv.New(t, http.NotFoundHandler(), nil, "")
	h.Env.Flags.Context = "nope"
	if err := h.Run(status.New(h.Env)); exitcode.Of(err) != exitcode.NotFound {
		t.Fatalf("exit %d: %v", exitcode.Of(err), err)
	}
}

func TestStatusTakesNoArgument(t *testing.T) {
	h := testenv.New(t, http.NotFoundHandler(), nil, "")
	if err := h.Run(status.New(h.Env), "x"); exitcode.Of(err) != exitcode.Usage {
		t.Fatalf("exit %d", exitcode.Of(err))
	}
}
