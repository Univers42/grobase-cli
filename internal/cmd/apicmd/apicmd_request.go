package apicmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// request turns the flags into the first request of the walk.
func (f apiFlags) request(e *env.Env, method, path string, mode httpx.Mode) (httpx.Request, error) {
	// The path is the caller's: never rewritten, only checked. "://" or a "//"
	// prefix would send the operator's credentials to another host.
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.Contains(path, "://") {
		return httpx.Request{}, exitcode.Usagef("PATH must be a path of this deployment, got %q", path)
	}
	if f.maxPages < 1 {
		return httpx.Request{}, exitcode.Usagef("--max-pages must be at least 1, got %d", f.maxPages)
	}
	p, query, err := splitPath(path)
	if err != nil {
		return httpx.Request{}, err
	}
	req := httpx.Request{Method: method, Path: p, Mode: mode, Query: query}
	if err := f.apply(e, &req); err != nil {
		return httpx.Request{}, err
	}
	return req, nil
}

// apply adds the headers and the body (or, for a GET, the query fields).
func (f apiFlags) apply(e *env.Env, req *httpx.Request) error {
	if err := addHeaders(req, f.headers); err != nil {
		return err
	}
	if f.input != "" {
		return f.setInput(e, req)
	}
	body, err := f.body()
	if err != nil || len(body) == 0 {
		return err
	}
	if req.Method == http.MethodGet {
		for k, v := range body {
			req.Query.Set(k, fmt.Sprint(v))
		}
		return nil
	}
	req.JSON = body
	return nil
}

// body collects -f and -F values into one JSON object.
func (f apiFlags) body() (map[string]any, error) {
	out := map[string]any{}
	for _, kv := range f.fields {
		k, v, err := splitKV(kv, "-f")
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	for _, kv := range f.typed {
		k, v, err := splitKV(kv, "-F")
		if err != nil {
			return nil, err
		}
		out[k] = typedValue(v)
	}
	return out, nil
}

// setInput sends a raw body: FILE, or stdin for "-". The content type stays
// unset unless -H gave one, so add 'Content-Type: ...' when the endpoint cares.
func (f apiFlags) setInput(e *env.Env, req *httpx.Request) error {
	var body []byte
	var err error
	if f.input == "-" {
		body, err = io.ReadAll(e.Stdin)
	} else {
		body, err = os.ReadFile(f.input)
	}
	if err != nil {
		return fmt.Errorf("read --input %s: %w", f.input, err)
	}
	req.Body = bytes.NewReader(body)
	req.ContentLength = int64(len(body))
	return nil
}

// addHeaders turns every -H into a header.
// Ponytail: these values are never redacted; the documented rule stands: no
// secrets in -H, use --mode and the credential store instead.
func addHeaders(req *httpx.Request, raw []string) error {
	for _, h := range raw {
		name, value, ok := strings.Cut(h, ":")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !ok || name == "" {
			return exitcode.Usagef("-H wants 'Name: value', got %q", h)
		}
		if req.Header == nil {
			req.Header = http.Header{}
		}
		req.Header.Add(name, value)
	}
	return nil
}

func splitKV(kv, flag string) (string, string, error) {
	k, v, ok := strings.Cut(kv, "=")
	if !ok || k == "" {
		return "", "", exitcode.Usagef("%s wants key=value, got %q", flag, kv)
	}
	return k, v, nil
}

// typedValue parses the scalars a flag can type: true, false, null and numbers.
// Everything else stays a string; the literal is kept so a big number is not
// squeezed through a float64.
func typedValue(v string) any {
	switch v {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if numberLiteral(v) {
		return json.Number(v)
	}
	return v
}

func numberLiteral(v string) bool {
	if v == "" {
		return false
	}
	switch v[0] {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-', '+', '.':
	default:
		return false
	}
	// json.Valid also accepts arrays and objects: those must stay strings here.
	return json.Valid([]byte(v))
}

// splitPath cuts the query string off a user-supplied PATH so extra values can
// be merged; the path itself is kept byte for byte.
func splitPath(path string) (string, url.Values, error) {
	p, raw, _ := strings.Cut(path, "?")
	q, err := url.ParseQuery(raw)
	if err != nil {
		return "", nil, exitcode.Usagef("PATH has a malformed query string: %v", err)
	}
	return p, q, nil
}

// parseMode maps --mode onto an httpx mode. The empty default lets the resolver
// pick the strongest stored credential.
func parseMode(s string) (httpx.Mode, error) {
	switch s {
	case "", "auto":
		return httpx.ModeAuto, nil
	case "user":
		return httpx.ModeUser, nil
	case "service":
		return httpx.ModeService, nil
	case "apikey":
		return httpx.ModeAPIKey, nil
	case "anon":
		return httpx.ModeAnon, nil
	case "none":
		return httpx.ModeNone, nil
	}
	return httpx.ModeAuto, exitcode.Usagef("unknown --mode %q (want auto, user, service, apikey, anon or none)", s)
}
