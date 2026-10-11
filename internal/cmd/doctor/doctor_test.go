package doctor_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Univers42/grobase-cli/internal/cmd/doctor"
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
