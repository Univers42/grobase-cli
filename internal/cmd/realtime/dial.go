package realtime

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// dial opens the realtime socket of the current context.
func dial(ctx context.Context, e *env.Env) (*websocket.Conn, error) {
	cl, _, err := e.Client()
	if err != nil {
		return nil, err
	}
	u, err := dialURL(ctx, e, cl)
	if err != nil {
		return nil, err
	}
	// The context's *http.Client carries the TLS trust (CA, pin, insecure), so
	// wss:// sockets verify exactly like the HTTPS calls do.
	c, resp, derr := websocket.Dial(ctx, u, &websocket.DialOptions{HTTPClient: cl.HTTP})
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if derr == nil {
		return c, nil
	}
	return nil, handshakeErr(e, resp, derr)
}

// dialURL turns the context URL into ws:// or wss:// and puts the credentials in
// the query, the way the realtime service reads them.
func dialURL(ctx context.Context, e *env.Env, cl *httpx.Client) (string, error) {
	apikey, bearer, err := credentials(ctx, e, cl)
	if err != nil {
		return "", err
	}
	scheme := "ws"
	if cl.Base.Scheme == "https" {
		scheme = "wss"
	}
	q := url.Values{}
	if apikey != "" {
		q.Set("apikey", apikey)
	}
	if bearer != "" {
		q.Set("access_token", bearer)
	}
	u := *cl.Base
	u.Scheme = scheme
	u.Path = strings.TrimSuffix(u.Path, "/") + wsPath
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// credentials asks the context's authorizer which headers it would send and
// reads them back, so the socket authenticates like every other command.
func credentials(ctx context.Context, e *env.Env, cl *httpx.Client) (apikey, bearer string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cl.Base.String(), nil)
	if err != nil {
		return "", "", fmt.Errorf("build credentials request: %w", err)
	}
	if err := cl.Auth.Authorize(ctx, req, httpx.ModeAuto); err != nil {
		return "", "", err
	}
	apikey = req.Header.Get("apikey")
	bearer = strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	// The dial URL carries both, so every error that quotes it must be scrubbed.
	e.Redact.Add(apikey)
	e.Redact.Add(bearer)
	return apikey, bearer, nil
}

// handshakeErr maps a rejected handshake onto the errors an HTTP call produces,
// so the exit codes and the "not enabled" wording stay uniform across the CLI.
func handshakeErr(e *env.Env, resp *http.Response, dialErr error) error {
	if resp == nil || resp.Request == nil {
		return e.Redact.Err(dialErr)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// A gateway or an HTTP fallback answered the handshake itself.
		return e.Redact.Err(fmt.Errorf("websocket: %s did not upgrade the connection: %w", wsPath, dialErr))
	}
	ae := &httpx.APIError{Method: http.MethodGet, Path: resp.Request.URL.Path, Status: resp.StatusCode}
	return e.Redact.Err(httpx.NotEnabled("realtime", ae))
}
