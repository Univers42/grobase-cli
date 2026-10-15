package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Univers42/grobase-cli/internal/redact"
)

// Mode selects which stored credential authenticates a request.
type Mode int

// Authentication modes.
const (
	// ModeAuto uses the best credential available: user session, API key, service role, anon.
	ModeAuto Mode = iota
	// ModeUser uses the GoTrue session (apikey=anon, bearer=access token).
	ModeUser
	// ModeService uses the service-role key as apikey and bearer.
	ModeService
	// ModeAPIKey uses an app API key (mbk_...) as bearer.
	ModeAPIKey
	// ModeAnon uses the anon (publishable) key.
	ModeAnon
	// ModeNone sends no credentials.
	ModeNone
	// ModeAutoAPIKeyFirst is ModeAuto with a stored API key ranked above the session,
	// for the query service (/query/v1), which accepts an app key only.
	ModeAutoAPIKeyFirst
)

// Authorizer decorates outgoing requests with credentials.
type Authorizer interface {
	// Authorize sets the auth headers on req for the mode.
	Authorize(ctx context.Context, req *http.Request, mode Mode) error
	// Refresh renews the session after a 401; it reports whether a retry is worthwhile.
	Refresh(ctx context.Context) (bool, error)
}

// Client talks to one grobase deployment.
type Client struct {
	Base      *url.URL
	HTTP      *http.Client
	Auth      Authorizer
	Debug     io.Writer
	Redact    *redact.Redactor
	UserAgent string
}

// Request describes one call. Path is already escaped (see Seg) and starts with "/".
type Request struct {
	Method string
	Path   string
	Query  url.Values
	Header http.Header
	JSON   any
	// Body streams as-is (uploads); such a request is never retried after a 401.
	Body io.Reader
	// ContentLength is the size of Body when known, so uploads are not chunked.
	ContentLength int64
	ContentType   string
	Mode          Mode
}

// TenantRequest is a service-role request on behalf of one tenant: the /admin/v1
// routes behind Kong read the tenant from X-Baas-Tenant-Id.
func TenantRequest(method, path, tenant string, body any) Request {
	return Request{
		Method: method, Path: path, Mode: ModeService, JSON: body,
		Header: http.Header{"X-Baas-Tenant-Id": {tenant}},
	}
}

// Seg escapes one path segment.
func Seg(s string) string { return url.PathEscape(s) }

// Do sends the request. A 2xx response is returned open for the caller to read and
// close; any other status becomes an *APIError. A 401 triggers one session refresh and retry
// when the body can be replayed.
func (c *Client) Do(ctx context.Context, r Request) (*http.Response, error) {
	body, replayable, err := encodeBody(r)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(ctx, r, body)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized && replayable && c.Auth != nil {
		if ok, rerr := c.Auth.Refresh(ctx); rerr == nil && ok {
			return c.send(ctx, r, body)
		}
	}
	return resp, err
}

// JSON sends the request and decodes a JSON answer into out (nil discards the body).
func (c *Client) JSON(ctx context.Context, r Request, out any) error {
	resp, err := c.Do(ctx, r)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if out == nil || len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%s %s: decode response: %w", r.Method, r.Path, err)
	}
	return nil
}

func encodeBody(r Request) ([]byte, bool, error) {
	switch {
	case r.Body != nil:
		return nil, false, nil
	case r.JSON != nil:
		b, err := json.Marshal(r.JSON)
		return b, true, err
	}
	return nil, true, nil
}

func (c *Client) newRequest(ctx context.Context, r Request, body []byte) (*http.Request, error) {
	full := strings.TrimRight(c.Base.String(), "/") + r.Path
	if len(r.Query) > 0 {
		full += "?" + r.Query.Encode()
	}
	rd := r.Body
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, full, rd)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if r.Body != nil && r.ContentLength > 0 {
		req.ContentLength = r.ContentLength
	}
	for k, v := range r.Header {
		req.Header[k] = append([]string(nil), v...)
	}
	ct := r.ContentType
	if ct == "" && r.JSON != nil {
		ct = "application/json"
	}
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if c.Auth != nil && r.Mode != ModeNone {
		if err := c.Auth.Authorize(ctx, req, r.Mode); err != nil {
			return nil, err
		}
	}
	return req, nil
}

func (c *Client) send(ctx context.Context, r Request, body []byte) (*http.Response, error) {
	req, err := c.newRequest(ctx, r, body)
	if err != nil {
		return nil, err
	}
	c.traceRequest(req, body)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, c.Redact.Err(fmt.Errorf("%s %s: %w", r.Method, r.Path, err))
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		c.traceResponse(resp)
		return resp, nil
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	c.traceBytes(resp, b)
	return nil, &APIError{
		Method: r.Method, Path: r.Path, Status: resp.StatusCode,
		Message: c.Redact.String(extractMessage(b)), Body: c.Redact.Bytes(b),
	}
}
