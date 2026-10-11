package doctor_test

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Univers42/grobase-cli/internal/cmd/doctor"
	"github.com/Univers42/grobase-cli/internal/config"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const apiKey = "mbk_doctor_secret_value"

func run(t *testing.T, status int, creds map[string]string) (string, error) {
	t.Helper()
	h := testenv.New(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/orgs" {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(status)
	}), creds, "")
	h.Now = time.Now()
	err := h.Run(doctor.New(h.Env))
	return h.Out.String() + h.Err.String(), err
}

func TestDoctorHealthyDeployment(t *testing.T) {
	out, err := run(t, 200, map[string]string{secret.KindAPIKey: apiKey})
	if err != nil {
		t.Fatalf("exit %d: %v\n%s", exitcode.Of(err), err, out)
	}
	for _, want := range []string{`"check": "config"`, "plaintext http", "stored: api_key", "not enabled on this deployment", "0 fail"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, apiKey) {
		t.Fatal("a credential value reached the report")
	}
}

func TestDoctorRejectedCredentialFails(t *testing.T) {
	for _, status := range []int{401, 403} {
		out, err := run(t, status, map[string]string{secret.KindAPIKey: apiKey})
		if err == nil || !strings.Contains(out, "run `grobase login`") {
			t.Errorf("%d: err=%v\n%s", status, err, out)
		}
	}
}

func TestDoctorNoCredentialSkipsAuth(t *testing.T) {
	out, err := run(t, 200, nil)
	if err != nil || !strings.Contains(out, "nothing stored") || !strings.Contains(out, `"result": "SKIP"`) {
		t.Fatalf("err=%v\n%s", err, out)
	}
}

func TestDoctorNoContextFails(t *testing.T) {
	h := testenv.New(t, http.NotFoundHandler(), nil, "")
	h.Env.Flags.Context = "nope"
	err := h.Run(doctor.New(h.Env))
	if err == nil || !strings.Contains(h.Out.String(), `"result": "SKIP"`) {
		t.Fatalf("err=%v\n%s", err, h.Out)
	}
}

// A context whose first URL is down (the LAN URL, away from home) is healthy through
// the alternate it falls back to: TLS is checked on the URL in use, not the first one,
// and the reach row says which URL that is.
func TestDoctorChecksTheAlternateInUse(t *testing.T) {
	h := testenv.New(t, http.NotFoundHandler(), nil, "")
	up := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/orgs" {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(200)
	}))
	t.Cleanup(up.Close)
	down := httptest.NewTLSServer(http.NotFoundHandler())
	downURL := down.URL
	down.Close()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: up.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	cf := &config.File{}
	if err := cf.Add(config.Context{Name: "away", URL: downURL, Alternates: []string{up.URL}, CAFile: ca}); err != nil {
		t.Fatal(err)
	}
	if err := h.Env.ConfigStore().Save(cf); err != nil {
		t.Fatal(err)
	}
	h.Now = time.Now()
	err := h.Run(doctor.New(h.Env))
	out := h.Out.String() + h.Err.String()
	if err != nil {
		t.Fatalf("exit %d: %v\n%s", exitcode.Of(err), err, out)
	}
	if !strings.Contains(out, "via "+up.URL) {
		t.Errorf("the reach row does not name the URL in use (%s)\n%s", up.URL, out)
	}
}
