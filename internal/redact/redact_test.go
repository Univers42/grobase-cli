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
		`{"password": "p\"w"}`:                       `{"password":"` + Mask + `"}`,
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

// The --debug leak the pattern-only redactor had: a secret in a shape no regex knows,
// under a key no list named, nested where the field regex never looked.
func TestStringMasksByKeyAtAnyDepth(t *testing.T) {
	r := New()
	in := `{"data":{"creds":[{"anon_key":"opaque-anon-123456","note":"keep"}]},` +
		`"meta":{"Private-Key":"pk-opaque-789","client_secret":{"v":"nested-opaque"}},` +
		`"otp":123456,"inline_dsn":"host=db password=pw-opaque","body":"{\"session_token\":\"quoted-opaque\"}",` +
		`"keys":[{"id":"k1","name":"ci"}],"verified":true,"x":null}`
	got := r.String(in)
	for _, leak := range []string{"opaque-anon-123456", "pk-opaque-789", "nested-opaque", "123456", "pw-opaque", "quoted-opaque"} {
		if strings.Contains(got, leak) {
			t.Errorf("leaked %q: %s", leak, got)
		}
	}
	for _, kept := range []string{`"note":"keep"`, `"id":"k1"`, `"name":"ci"`, `"verified":true`, `"x":null`} {
		if !strings.Contains(got, kept) {
			t.Errorf("over-masked, lost %s: %s", kept, got)
		}
	}
}

func TestStringMasksFreeTextShapes(t *testing.T) {
	r := New()
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIEow\n-----END RSA PRIVATE KEY-----"
	cases := map[string]string{
		"dial postgres://app:s3cret@db:5432/app":     "dial postgres://app:" + Mask + "@db:5432/app",
		"got Authorization: Bearer opaque.token.v1x": "got Authorization: Bearer " + Mask,
		"key " + pem + " end":                        "key " + Mask + " end",
		"https://host:8443/path@x stays":             "https://host:8443/path@x stays",
		// A JSON body clipped mid-document is no document: the field regex still applies.
		`{"password":"cut-opaque","more":"...(truncated)`: `{"password":"` + Mask + `","more":"...(truncated)`,
	}
	for in, want := range cases {
		if got := r.String(in); got != want {
			t.Errorf("String(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSensitiveKey(t *testing.T) {
	for k, want := range map[string]bool{
		"apikey": true, "anon_key": true, "SERVICE-ROLE-KEY": true, "x-api-key": true, "accessToken": true,
		"client_secret": true, "inline_dsn": true, "otp": true, "credential_ref": true,
		"keys": false, "monkeys": false, "email": false, "id": false, "name": false, "footprint": false,
	} {
		if sensitiveKey(k) != want {
			t.Errorf("sensitiveKey(%q) = %v, want %v", k, !want, want)
		}
	}
}
