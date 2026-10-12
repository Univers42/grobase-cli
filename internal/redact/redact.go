// Package redact scrubs credentials from anything that is about to be printed or
// logged: HTTP headers, request/response bodies and error strings.
//
// Three layers, in order: every secret value the CLI has loaded or read (Add), exactly;
// in a JSON document, every value under a key that names a credential, at any depth
// (sensitiveKey), whatever its shape; then the shapes a credential takes in free text
// (JWTs, mbk_ keys, URL passwords, Bearer tokens, PEM private keys, JSON-ish fields in a
// string that is not one document).
//
// Caveat: a secret the CLI never held, under an innocuous key (`{"value":"…"}`) or in
// non-JSON text in no recognised shape, still passes. A JSON body cut short (a trace
// clipped at its limit) falls back to the free-text layer. Over-masking is the chosen
// failure: a key like credential_ref or token_type loses its value too.
package redact

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// Mask replaces a redacted value.
const Mask = "[REDACTED]"

// Redactor masks secrets. The zero value is not usable; call New.
type Redactor struct {
	shapes []shape
	mu     sync.RWMutex
	known  []string
}

// shape is one free-text pattern and what replaces a match.
type shape struct {
	re   *regexp.Regexp
	repl string
}

// New returns a Redactor that also masks every value in known.
func New(known ...string) *Redactor {
	r := &Redactor{shapes: []shape{
		{regexp.MustCompile(`eyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*`), Mask},
		{regexp.MustCompile(`mbk_[A-Za-z0-9_-]{6,}`), Mask},
		{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), Mask},
		{regexp.MustCompile(`\b([a-zA-Z][a-zA-Z0-9+.-]*://[^:/@\s"']*:)[^@\s"'/]+@`), "${1}" + Mask + "@"},
		{regexp.MustCompile(`(?i)\b(bearer\s+)[A-Za-z0-9._~+/=-]{8,}`), "${1}" + Mask},
		{
			regexp.MustCompile(`(?i)("(?:access_token|refresh_token|id_token|token|password|new_password|` +
				`secret|api_key|apikey|key|service_role|connection_string|dsn|otp|token_hash)"\s*:\s*)"(?:[^"\\]|\\.)*"`),
			`$1"` + Mask + `"`,
		},
	}}
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

// String masks secrets inside s. A string that is one JSON document comes back
// re-encoded, its object keys sorted.
func (r *Redactor) String(s string) string {
	r.mu.RLock()
	for _, k := range r.known {
		s = strings.ReplaceAll(s, k, Mask)
	}
	r.mu.RUnlock()
	if masked, ok := maskDocument(s); ok {
		s = masked
	}
	for _, sh := range r.shapes {
		s = sh.re.ReplaceAllString(s, sh.repl)
	}
	return s
}

// Bytes masks secrets inside b.
func (r *Redactor) Bytes(b []byte) []byte { return []byte(r.String(string(b))) }

// maskDocument re-encodes s with every value under a credential-named key masked; ok is
// false unless s is exactly one JSON object or array.
func maskDocument(s string) (string, bool) {
	t := strings.TrimSpace(s)
	if t == "" || (t[0] != '{' && t[0] != '[') {
		return "", false
	}
	dec := json.NewDecoder(strings.NewReader(t))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil || dec.Decode(&struct{}{}) != io.EOF {
		return "", false
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if enc.Encode(maskValue(v, false)) != nil {
		return "", false
	}
	return strings.TrimRight(b.String(), "\n"), true
}

// maskValue masks every scalar below a sensitive key; a string that is itself a JSON
// document (a body quoted inside a body) is masked the same way.
func maskValue(v any, sensitive bool) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			x[k] = maskValue(val, sensitive || sensitiveKey(k))
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = maskValue(val, sensitive)
		}
		return x
	case nil, bool:
		return x
	case string:
		if sensitive {
			return Mask
		}
		if inner, ok := maskDocument(x); ok {
			return inner
		}
		return x
	}
	if sensitive {
		return Mask
	}
	return v
}

// sensitiveWords are the words that make a JSON key name a credential.
var sensitiveWords = []string{
	"token", "secret", "password", "passwd", "passphrase", "credential", "authorization",
	"cookie", "signature", "connectionstring", "privatekey",
}

// sensitiveKey reports whether a JSON key names a credential: lowercased, with `_` and
// `-` dropped, it holds one of sensitiveWords, ends in key or dsn (apikey, anon_key,
// inline_dsn), or is otp or salt.
func sensitiveKey(k string) bool {
	n := strings.Map(func(c rune) rune {
		if c == '_' || c == '-' {
			return -1
		}
		return unicode.ToLower(c)
	}, k)
	if n == "otp" || n == "salt" || strings.HasSuffix(n, "key") || strings.HasSuffix(n, "dsn") {
		return true
	}
	for _, w := range sensitiveWords {
		if strings.Contains(n, w) {
			return true
		}
	}
	return false
}

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
