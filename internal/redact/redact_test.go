package redact

import (
	"net/http"
	"strings"
	"testing"
)

const (
	jwtSample = "eyJhbGciOiJIUzI1NiJ9.eyJyb2xlIjoiYW5vbiJ9.c2lnbmF0dXJl"
	mbkSample = "mbk_AbCdEf123456_xyz"
)

func TestString(t *testing.T) {
	r := New("opaque-known-secret")
	cases := map[string]string{
		"bearer " + jwtSample:                        "bearer " + Mask,
		"key " + mbkSample + " end":                  "key " + Mask + " end",
		`{"access_token":"abc123xyz","x":1}`:         `{"access_token":"` + Mask + `","x":1}`,
		`{"password": "p\"w"}`:                       `{"password": "` + Mask + `"}`,
		"leak opaque-known-secret here":              "leak " + Mask + " here",
		"nothing to see":                             "nothing to see",
		`{"email":"a@b.c","refresh_token":"zzzzzz"}`: `{"email":"a@b.c","refresh_token":"` + Mask + `"}`,
	}
	for in, want := range cases {
		if got := r.String(in); got != want {
			t.Errorf("String(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHeader(t *testing.T) {
	r := New()
	h := http.Header{}
	h.Set("Authorization", "Bearer "+jwtSample)
	h.Set("apikey", jwtSample)
	h.Set("X-Note", "token "+mbkSample)
	h.Set("Content-Type", "application/json")
	out := r.Header(h)
	for _, name := range []string{"Authorization", "Apikey"} {
		if out.Get(name) != Mask {
			t.Errorf("%s = %q", name, out.Get(name))
		}
	}
	if strings.Contains(out.Get("X-Note"), "mbk_") {
		t.Errorf("X-Note leaked: %q", out.Get("X-Note"))
	}
	if out.Get("Content-Type") != "application/json" {
		t.Errorf("content-type altered")
	}
}

func TestErr(t *testing.T) {
	r := New()
	err := r.Err(&testErr{"failed with " + jwtSample})
	if strings.Contains(err.Error(), "eyJ") {
		t.Fatalf("leaked: %v", err)
	}
	if r.Err(nil) != nil {
		t.Fatal("nil must stay nil")
	}
}

type testErr struct{ s string }

func (e *testErr) Error() string { return e.s }
