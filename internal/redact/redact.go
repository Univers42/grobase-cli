// Package redact scrubs credentials from anything that is about to be printed or
// logged: HTTP headers, request/response bodies and error strings.
//
// Ponytail: detection is pattern-based (JWTs, mbk_ keys, well-known JSON field
// names) plus an explicit list of known secret values. A secret in an unrecognised
// shape that was never registered via Add (e.g. a bare opaque token in a custom
// field) passes through unmasked; callers must Add every secret they load.
package redact

import (
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// Mask replaces a redacted value.
const Mask = "[REDACTED]"

// Redactor masks secrets. The zero value is not usable; call New.
type Redactor struct {
	jwt   *regexp.Regexp
	mbk   *regexp.Regexp
	field *regexp.Regexp
	mu    sync.RWMutex
	known []string
}

// New returns a Redactor that also masks every value in known.
func New(known ...string) *Redactor {
	r := &Redactor{
		jwt: regexp.MustCompile(`eyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*`),
		mbk: regexp.MustCompile(`mbk_[A-Za-z0-9_-]{6,}`),
		field: regexp.MustCompile(`(?i)("(?:access_token|refresh_token|id_token|token|password|new_password|` +
			`secret|api_key|apikey|key|service_role|connection_string|otp|token_hash)"\s*:\s*)"(?:[^"\\]|\\.)*"`),
	}
	for _, k := range known {
		r.Add(k)
	}
	return r
}

// Add registers a secret value to be masked wherever it appears.
func (r *Redactor) Add(secret string) {
	if len(secret) < 6 {
		return // masking tiny strings would shred unrelated output
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.known = append(r.known, secret)
}

// String masks secrets inside s.
func (r *Redactor) String(s string) string {
	r.mu.RLock()
	for _, k := range r.known {
		s = strings.ReplaceAll(s, k, Mask)
	}
	r.mu.RUnlock()
	s = r.jwt.ReplaceAllString(s, Mask)
	s = r.mbk.ReplaceAllString(s, Mask)
	return r.field.ReplaceAllString(s, `$1"`+Mask+`"`)
}

// Bytes masks secrets inside b.
func (r *Redactor) Bytes(b []byte) []byte { return []byte(r.String(string(b))) }

func sensitiveHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Authorization", "Apikey", "Cookie", "Set-Cookie", "Proxy-Authorization", "X-Api-Key":
		return true
	}
	return false
}

// Header returns a copy of h with credential headers fully masked and the rest scrubbed.
func (r *Redactor) Header(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for name, vals := range h {
		for _, v := range vals {
			if sensitiveHeader(name) {
				out.Add(name, Mask)
			} else {
				out.Add(name, r.String(v))
			}
		}
	}
	return out
}

// Error is an error whose message is scrubbed.
type Error struct {
	Err error
	r   *Redactor
}

// Error implements error.
func (e *Error) Error() string { return e.r.String(e.Err.Error()) }

// Unwrap exposes the underlying error to errors.Is/As.
func (e *Error) Unwrap() error { return e.Err }

// Err wraps err so that printing it cannot leak a secret; nil stays nil.
func (r *Redactor) Err(err error) error {
	if err == nil {
		return nil
	}
	return &Error{Err: err, r: r}
}
