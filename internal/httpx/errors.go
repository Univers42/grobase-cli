package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// APIError is a non-2xx answer from the server.
type APIError struct {
	Method  string
	Path    string
	Status  int
	Message string
	Body    []byte
}

// Error implements error.
func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = strings.ToLower(http.StatusText(e.Status))
	}
	s := fmt.Sprintf("%s %s: %d %s", e.Method, e.Path, e.Status, msg)
	if e.NotFound() {
		s += " (the resource does not exist, or the feature is not enabled on this deployment)"
	}
	return s
}

// NotFound reports a 404, which on flag-gated routes means "not enabled here".
func (e *APIError) NotFound() bool { return e.Status == http.StatusNotFound }

// Unauthorized reports 401 or 403.
func (e *APIError) Unauthorized() bool {
	return e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden
}

// extractMessage pulls a human message out of the JSON error shapes used by
// GoTrue (msg, error_description), PostgREST (message, hint) and the control plane (error).
func extractMessage(body []byte) string {
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return strings.TrimSpace(truncate(string(body), 200))
	}
	for _, k := range []string{"msg", "message", "error_description", "error", "detail"} {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
		if obj, ok := m[k].(map[string]any); ok {
			if v, ok := obj["message"].(string); ok && v != "" {
				return v
			}
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// NotEnabled relabels a 404 on a feature's root route as "not enabled on this
// deployment" (flag-gated routes answer 404 when off), keeping the cause for exit-code
// mapping. Ponytail: it cannot tell a disabled feature from a wrong path; call it only
// on collection roots, never on id-scoped routes.
func NotEnabled(what string, err error) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.NotFound() {
		return fmt.Errorf("%s: not enabled on this deployment: %w", what, err)
	}
	return err
}
