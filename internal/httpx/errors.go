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
		s += " (" + e.Miss().explain() + ")"
	}
	return s
}

// NotFound reports a 404; Miss says which of the three a 404 is.
func (e *APIError) NotFound() bool { return e.Status == http.StatusNotFound }

// Miss says who answered a 404, which is what tells a missing feature from a missing thing.
type Miss int

const (
	// MissUnknown is a 404 whose body names no author: a missing thing, or a missing feature.
	MissUnknown Miss = iota
	// MissNoRoute is Kong matching no route: this deployment does not serve the path at all.
	MissNoRoute
	// MissNoHandler is the service behind a route having no handler for the path: the feature
	// is switched off in it (grobase keeps an off feature byte-identical to an unknown path,
	// e.g. TENANT_SELFSERVE_ENABLED), or this CLI and the server disagree on the path.
	MissNoHandler
)

// kongNoRoute is the message of Kong's proxy 404 when no route matches.
const kongNoRoute = "no Route matched with those values"

// Miss classifies a 404 by who wrote its body; any other status is MissUnknown.
// Caveat: it reads the body text, recognising Kong's no-route message, Go's "404 page not
// found" and Express/NestJS "Cannot GET /x". Every other 404 is MissUnknown, JSON ones
// included: GoTrue and PostgREST answer an unknown path and a missing row alike. And
// "switched off" cannot be told from "this CLI and the server disagree on the path" from
// outside: grobase answers both with the same bytes on purpose.
func (e *APIError) Miss() Miss {
	if !e.NotFound() {
		return MissUnknown
	}
	body := strings.TrimSpace(string(e.Body))
	switch {
	case e.Message == kongNoRoute:
		return MissNoRoute
	case body == "404 page not found", strings.HasPrefix(e.Message, "Cannot "), strings.Contains(body, "<pre>Cannot "):
		return MissNoHandler
	}
	return MissUnknown
}

// explain is the parenthesis a 404's message ends with.
func (m Miss) explain() string {
	switch m {
	case MissNoRoute:
		return "the gateway has no route for this path: this deployment does not serve the feature"
	case MissNoHandler:
		return "the service has no handler for this path: the feature is switched off on this deployment, " +
			"or this CLI and the server disagree on the path"
	}
	return "the resource does not exist, or the feature is not enabled on this deployment"
}

// FeatureState is what a 404 on a feature's own route says about the feature.
func (m Miss) FeatureState() string {
	switch m {
	case MissNoRoute:
		return "not deployed"
	case MissNoHandler:
		return "switched off"
	}
	return "not enabled"
}

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

// NotEnabled names the feature in a 404 on its collection root, saying how it is absent
// (no gateway route, no handler, or a body that does not say), and keeps the cause for
// exit-code mapping. Call it only on collection roots: on an id-scoped route a 404 is
// usually the id.
func NotEnabled(what string, err error) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.NotFound() {
		return fmt.Errorf("%s: %s: %w", what, apiErr.Miss().absent(), err)
	}
	return err
}

// absent is how NotEnabled says a feature is missing; the unclassified wording is the one
// scripts already match.
func (m Miss) absent() string {
	switch m {
	case MissNoRoute:
		return "not deployed on this gateway"
	case MissNoHandler:
		return "switched off on this deployment"
	}
	return "not enabled on this deployment"
}
