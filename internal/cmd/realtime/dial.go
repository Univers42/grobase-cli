package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/Univers42/grobase-cli/internal/auth"
	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// dial opens the realtime socket of the current context and completes the AUTH
// handshake: the first frame carries the session token and the gateway answers
// AUTH_OK before the command may send anything else.
func dial(ctx context.Context, e *env.Env, timeout time.Duration) (*websocket.Conn, error) {
	cl, _, err := e.Client()
	if err != nil {
		return nil, err
	}
	apikey, token, err := credentials(ctx, e, cl)
	if err != nil {
		return nil, err
	}
	c, resp, derr := websocket.Dial(ctx, dialURL(cl, apikey), &websocket.DialOptions{HTTPClient: cl.HTTP})
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if derr != nil {
		return nil, handshakeErr(e, resp, derr)
	}
	if err := authenticate(ctx, e, c, token, timeout); err != nil {
		_ = c.CloseNow()
		return nil, err
	}
	return c, nil
}

// credentials resolves the apikey for the dial URL and the session token for the
// AUTH frame, the same bearer the HTTP calls send. Realtime needs a logged-in
// user: an anon-only context is a usage error before any connection is made.
func credentials(ctx context.Context, e *env.Env, cl *httpx.Client) (apikey, token string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cl.Base.String(), nil)
	if err != nil {
		return "", "", fmt.Errorf("build credentials request: %w", err)
	}
	if err := cl.Auth.Authorize(ctx, req, httpx.ModeUser); err != nil {
		if errors.Is(err, auth.ErrNoCredential) {
			return "", "", exitcode.Usagef("realtime needs a logged-in user session: run `grobase login` first")
		}
		return "", "", err
	}
	apikey = req.Header.Get("apikey")
	token = strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	e.Redact.Add(apikey)
	e.Redact.Add(token)
	return apikey, token, nil
}

// dialURL turns the context URL into ws:// or wss:// and keeps only the apikey in
// the query; the bearer travels in the AUTH frame, never in the URL where
// proxies log it.
func dialURL(cl *httpx.Client, apikey string) string {
	scheme := "ws"
	if cl.Base.Scheme == "https" {
		scheme = "wss"
	}
	q := url.Values{}
	if apikey != "" {
		q.Set("apikey", apikey)
	}
	u := *cl.Base
	u.Scheme = scheme
	u.Path = strings.TrimSuffix(u.Path, "/") + wsPath
	u.RawQuery = q.Encode()
	return u.String()
}

// authenticate sends the AUTH frame and waits for AUTH_OK, bounded by timeout
// when positive. A server ERROR is an auth failure carrying the server's code
// and message; any other first frame or a close is a plain error.
func authenticate(ctx context.Context, e *env.Env, c *websocket.Conn, token string, timeout time.Duration) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if err := writeJSON(ctx, c, authFrame{Type: "AUTH", Token: token}); err != nil {
		return e.Redact.Err(fmt.Errorf("send AUTH: %w", err))
	}
	typ, b, err := c.Read(ctx)
	if err != nil {
		return e.Redact.Err(fmt.Errorf("read AUTH_OK: %w", err))
	}
	var f envelope
	if typ != websocket.MessageText || json.Unmarshal(b, &f) != nil {
		return e.Redact.Err(errors.New("malformed first frame from the gateway"))
	}
	switch f.Type {
	case "AUTH_OK":
		return nil
	case "ERROR":
		return e.Redact.Err(fmt.Errorf("authentication failed (%s): %s: %w", f.Code, f.Message, auth.ErrNoCredential))
	default:
		return e.Redact.Err(fmt.Errorf("unexpected first frame %q from the gateway", f.Type))
	}
}

// authFrame is the first frame on the wire: the session token the gateway
// authenticates the socket with.
type authFrame struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// envelope is the part of a gateway frame that every frame type shares.
type envelope struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
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
