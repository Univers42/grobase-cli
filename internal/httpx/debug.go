package httpx

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxTraceBody = 64 << 10

func (c *Client) tracef(format string, a ...any) {
	if c.Debug != nil {
		_, _ = fmt.Fprintf(c.Debug, format+"\n", a...)
	}
}

func (c *Client) traceHeaders(prefix string, h http.Header) {
	for name, vals := range c.Redact.Header(h) {
		for _, v := range vals {
			c.tracef("%s %s: %s", prefix, name, v)
		}
	}
}

// traceRequest prints the request with every credential masked.
func (c *Client) traceRequest(req *http.Request, body []byte) {
	if c.Debug == nil {
		return
	}
	c.tracef("> %s %s", req.Method, c.Redact.String(req.URL.String()))
	c.traceHeaders(">", req.Header)
	if len(body) > 0 && isTextual(req.Header.Get("Content-Type")) {
		c.tracef("> body: %s", clip(c.Redact.Bytes(body)))
	}
}

// traceResponse prints status and headers, and buffers small textual bodies to print them.
func (c *Client) traceResponse(resp *http.Response) {
	if c.Debug == nil {
		return
	}
	c.tracef("< %s", resp.Status)
	c.traceHeaders("<", resp.Header)
	if !isTextual(resp.Header.Get("Content-Type")) || resp.ContentLength < 0 || resp.ContentLength > maxTraceBody {
		return
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(b))
	if err == nil && len(b) > 0 {
		c.tracef("< body: %s", clip(c.Redact.Bytes(b)))
	}
}

func (c *Client) traceBytes(resp *http.Response, b []byte) {
	if c.Debug == nil {
		return
	}
	c.tracef("< %s", resp.Status)
	c.traceHeaders("<", resp.Header)
	if len(b) > 0 {
		c.tracef("< body: %s", clip(c.Redact.Bytes(b)))
	}
}

// clip bounds a body already redacted: cut first, a JSON body would lose the structure
// the redactor reads.
func clip(b []byte) string {
	if len(b) > maxTraceBody {
		return string(b[:maxTraceBody]) + "...(truncated)"
	}
	return string(b)
}

func isTextual(ct string) bool {
	return ct == "" || strings.Contains(ct, "json") || strings.HasPrefix(ct, "text/") || strings.Contains(ct, "x-www-form-urlencoded")
}
