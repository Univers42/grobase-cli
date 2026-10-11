package users

import (
	"encoding/json"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// serviceClient returns the authenticated client of the current context.
func serviceClient(e *env.Env) (*httpx.Client, error) {
	cl, _, err := e.Client()
	return cl, err
}

// adminReq builds one admin request: the mode is fixed so a command can never
// accidentally talk to the API with a weaker credential.
func adminReq(method, path string) httpx.Request {
	return httpx.Request{Method: method, Path: path, Mode: httpx.ModeService}
}

// readPassword returns the password read from stdin or from the environment, or ""
// when the caller did not ask for one. The policy is checked here, before any request
// is sent, so a weak password never reaches the server.
func readPassword(e *env.Env, fromStdin bool) (string, error) {
	const envVar = "GROBASE_USER_PASSWORD"
	if !fromStdin && e.Getenv(envVar) == "" {
		return "", nil
	}
	pw, err := e.ReadSecret("Password: ", fromStdin, envVar)
	if err != nil {
		return "", err
	}
	return pw, CheckPassword(pw)
}

// decodeMetadata parses a --metadata JSON object; an empty flag value means "absent".
func decodeMetadata(s string) (map[string]any, error) {
	if s == "" {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, exitcode.Usagef("--metadata must be a JSON object: %v", err)
	}
	return m, nil
}
