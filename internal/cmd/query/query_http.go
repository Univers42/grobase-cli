package query

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// runFlags carries `query run`.
type runFlags struct {
	db, sql, file string
}

// crudFlags carries `query crud`.
type crudFlags struct {
	db, action, resource string
	filter, data         string
	dataFile             string
	dataStdin            bool
}

// crudActions are the actions the query service accepts for a table.
var crudActions = map[string]bool{"select": true, "insert": true, "update": true, "delete": true}

// validate checks the flags before any request is built.
func (f crudFlags) validate() error {
	if f.db == "" {
		return exitcode.Usagef("--db is required")
	}
	if !crudActions[f.action] {
		return exitcode.Usagef("--action must be one of select, insert, update or delete (got %q)", f.action)
	}
	if f.resource == "" {
		return exitcode.Usagef("--resource is required")
	}
	// insert and update write, so they need something to write.
	if (f.action == "insert" || f.action == "update") && f.data == "" && f.dataFile == "" && !f.dataStdin {
		return exitcode.Usagef("--%s needs rows: pass --data, --data-file or --data-stdin", f.action)
	}
	return nil
}

// statement returns the SQL to run: --sql wins, then --file ("-" is stdin).
func (f runFlags) statement(e *env.Env) (string, error) {
	switch {
	case f.sql != "":
		return f.sql, nil
	case f.file == "-":
		return readAll(e.Stdin)
	case f.file != "":
		return readFile(f.file)
	}
	return "", exitcode.Usagef("pass --sql, --file FILE, or --file - to read the statement from stdin")
}

// dataText returns the JSON to write, or "" when the call carries none.
func (f crudFlags) dataText(e *env.Env) (string, error) {
	switch {
	case f.data != "":
		return f.data, nil
	case f.dataFile == "-":
		return readAll(e.Stdin)
	case f.dataFile != "":
		return readFile(f.dataFile)
	case f.dataStdin:
		return readAll(e.Stdin)
	}
	return "", nil
}

// payload builds the payload object: filter and data are optional JSON values.
func (f crudFlags) payload(e *env.Env) (map[string]any, error) {
	payload := map[string]any{}
	if f.filter != "" {
		v, err := parseJSON("--filter", f.filter)
		if err != nil {
			return nil, err
		}
		payload["filter"] = v
	}
	raw, err := f.dataText(e)
	if err != nil {
		return nil, err
	}
	if raw != "" {
		v, err := parseJSON("--data", raw)
		if err != nil {
			return nil, err
		}
		payload["data"] = v
	}
	return payload, nil
}

// runSQL posts the statement to the query service's SQL runner and prints the rows that
// came back. The runner is `POST /query/v1/{db}/sql-ro`, grobase's one raw-SQL route: the
// execute route has no raw action and refuses an empty resource, so the old
// `{"action":"raw","resource":""}` body was a 400 Validation Error on every grobase
// (2026-10-12, the b2b VM; it hid behind the WAF's 403 until rule 10140).
func runSQL(ctx context.Context, e *env.Env, cl *httpx.Client, f runFlags, mode httpx.Mode) error {
	sql, err := f.statement(e)
	if err != nil {
		return err
	}
	req := httpx.Request{
		Method: "POST", Path: sqlPath(f.db), Mode: mode,
		JSON: map[string]any{"sql": sql},
	}
	var out any
	if err := cl.JSON(ctx, req, &out); err != nil {
		return httpx.NotEnabled("query run", err)
	}
	return printResult(e, out)
}

// sqlPath is the SQL runner's route for one database.
func sqlPath(db string) string {
	return "/query/v1/" + url.PathEscape(db) + "/sql-ro"
}

// runCrud maps the action, resource, filter and data onto the execute route.
func runCrud(ctx context.Context, e *env.Env, cl *httpx.Client, f crudFlags, mode httpx.Mode) error {
	payload, err := f.payload(e)
	if err != nil {
		return err
	}
	req := httpx.Request{
		Method: "POST", Path: execPath, Mode: mode,
		JSON: map[string]any{"database_id": f.db, "action": f.action, "resource": f.resource, "payload": payload},
	}
	var out any
	if err := cl.JSON(ctx, req, &out); err != nil {
		return httpx.NotEnabled("query crud", err)
	}
	return printResult(e, out)
}

// parseMode maps --mode onto an httpx mode. auto lets the resolver pick the strongest
// stored credential the query service accepts, so an API key outranks the session.
func parseMode(s string) (httpx.Mode, error) {
	switch s {
	case "", "auto":
		return httpx.ModeAutoAPIKeyFirst, nil
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

// printResult renders a query answer: a bare array of rows, an object wrapping
// them under "rows" or "data", or whatever else the server sent.
func printResult(e *env.Env, out any) error {
	if m, ok := out.(map[string]any); ok {
		for _, key := range []string{"rows", "data"} {
			if rows, ok := m[key].([]any); ok {
				return e.Printer().Print(rows)
			}
		}
	}
	return e.Printer().Print(out)
}

// parseJSON decodes a flag value carrying a JSON document.
func parseJSON(what, raw string) (any, error) {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, exitcode.Usagef("invalid %s JSON: %v", what, err)
	}
	return v, nil
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // G304: the file is chosen by the operator, like a shell redirect
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return strings.TrimSpace(string(b)), nil
}

func readAll(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}
