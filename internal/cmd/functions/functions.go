// Package functions implements the `grobase functions` command.
package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// New returns the `functions` command.
func New(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "functions",
		Short: "List, deploy, invoke and read the logs of edge functions",
		Args:  exitcode.Args(cobra.NoArgs),
	}
	c.AddCommand(newListCmd(e), newDeployCmd(e), newInvokeCmd(e), newLogsCmd(e))
	return c
}

func fnPath(name string, rest ...string) string {
	p := "/functions/v1/" + httpx.Seg(name)
	for _, r := range rest {
		p += "/" + r
	}
	return p
}

func newListCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the function triggers of the deployment (needs the service role)",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			var out any
			req := httpx.Request{Method: http.MethodGet, Path: "/admin/v1/function-triggers", Mode: httpx.ModeService}
			if err := cl.JSON(cmd.Context(), req, &out); err != nil {
				return httpx.NotEnabled("functions", err)
			}
			return e.Printer().Print(out)
		},
	}
}

func newDeployCmd(e *env.Env) *cobra.Command {
	var file, contentType string
	cmd := &cobra.Command{
		Use:   "deploy NAME --file FILE",
		Short: "Upload the source of a function (streamed from FILE)",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return exitcode.Usagef("--file is required")
			}
			return deploy(cmd.Context(), e, args[0], file, contentType)
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "source file to upload (required)")
	cmd.Flags().StringVar(&contentType, "content-type", "application/javascript", "Content-Type of the upload")
	return cmd
}

func deploy(ctx context.Context, e *env.Env, name, file, contentType string) error {
	fh, err := os.Open(filepath.Clean(file))
	if err != nil {
		return err
	}
	defer func() { _ = fh.Close() }()
	st, err := fh.Stat()
	if err != nil {
		return err
	}
	if st.IsDir() {
		return exitcode.Usagef("%s is a directory", file)
	}
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	req := httpx.Request{
		Method: http.MethodPut, Path: fnPath(name), Body: fh,
		ContentLength: st.Size(), ContentType: contentType,
	}
	var out map[string]any
	if err := cl.JSON(ctx, req, &out); err != nil {
		return err
	}
	if len(out) == 0 {
		out = map[string]any{"deployed": name}
	}
	return e.Printer().Print(out)
}

func newInvokeCmd(e *env.Env) *cobra.Command {
	var data, method string
	var dataStdin bool
	cmd := &cobra.Command{
		Use:   "invoke NAME [--data JSON | --data-stdin]",
		Short: "Call a function and print its response",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := invokeBody(e, data, dataStdin)
			if err != nil {
				return err
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			req := httpx.Request{Method: method, Path: fnPath(args[0], "invoke")}
			if body != nil {
				req.Body, req.ContentLength, req.ContentType = bytes.NewReader(body), int64(len(body)), "application/json"
			}
			return printBody(cmd.Context(), e, cl, req)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&data, "data", "", "request body as JSON")
	fl.BoolVar(&dataStdin, "data-stdin", false, "read the JSON request body from stdin")
	fl.StringVar(&method, "method", http.MethodPost, "HTTP method")
	return cmd
}

func invokeBody(e *env.Env, data string, fromStdin bool) ([]byte, error) {
	if fromStdin && data != "" {
		return nil, exitcode.Usagef("--data and --data-stdin are mutually exclusive")
	}
	if fromStdin {
		b, err := io.ReadAll(e.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		data = string(bytes.TrimSpace(b))
	}
	if data == "" {
		return nil, nil
	}
	if !json.Valid([]byte(data)) {
		return nil, exitcode.Usagef("the request body is not valid JSON")
	}
	return []byte(data), nil
}

// printBody writes a JSON answer through the printer and anything else verbatim.
func printBody(ctx context.Context, e *env.Env, cl *httpx.Client, req httpx.Request) error {
	resp, err := cl.Do(ctx, req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	var v any
	if json.Unmarshal(raw, &v) == nil {
		return e.Printer().Print(v)
	}
	_, err = e.Stdout.Write(raw)
	return err
}

// Caveat: when the gateway has a logs route, a 404 cannot tell a missing function from a
// runtime without logs; both read "not enabled" (a missing gateway route reads "not
// deployed"). Check `functions list` to tell them apart.
func newLogsCmd(e *env.Env) *cobra.Command {
	var tail int
	cmd := &cobra.Command{
		Use:   "logs NAME [--tail N]",
		Short: "Show the recent log lines of a function",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tail < 0 {
				return exitcode.Usagef("--tail must not be negative")
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			q := url.Values{"tail": {strconv.Itoa(tail)}}
			err = printBody(cmd.Context(), e, cl, httpx.Request{Method: http.MethodGet, Path: fnPath(args[0], "logs"), Query: q})
			return httpx.NotEnabled("function logs", err)
		},
	}
	cmd.Flags().IntVar(&tail, "tail", 100, "number of lines")
	return cmd
}
