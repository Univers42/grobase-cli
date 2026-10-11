package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// bucketSpec is the POST /storage/v1/bucket body.
type bucketSpec struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Public bool   `json:"public"`
}

func newBucketsCmd(e *env.Env) *cobra.Command {
	c := &cobra.Command{Use: "buckets", Short: "Manage the buckets of the current tenant"}
	c.AddCommand(newBucketsListCmd(e), newBucketsCreateCmd(e), newBucketsDeleteCmd(e))
	return c
}

func newBucketsListCmd(e *env.Env) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the buckets of the current tenant",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			rows, err := listBuckets(cmd.Context(), cl)
			if err != nil {
				return err
			}
			return e.Printer().Print(rows, "id", "name", "public", "created_at")
		},
	}
}

func listBuckets(ctx context.Context, cl *httpx.Client) ([]map[string]any, error) {
	var raw json.RawMessage
	req := httpx.Request{Method: http.MethodGet, Path: bucketPath()}
	if err := cl.JSON(ctx, req, &raw); err != nil {
		// The buckets collection is the root of the storage API: a 404 means the
		// deployment has storage switched off.
		return nil, httpx.NotEnabled("storage", err)
	}
	return decodeRows(raw, "buckets")
}

func newBucketsCreateCmd(e *env.Env) *cobra.Command {
	var public bool
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a bucket",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if err := checkBucket(name); err != nil {
				return err
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			out, err := createBucket(cmd.Context(), cl, name, public)
			if err != nil {
				return err
			}
			return printResult(e, out)
		},
	}
	cmd.Flags().BoolVar(&public, "public", false, "serve the objects without a signed URL")
	return cmd
}

func createBucket(ctx context.Context, cl *httpx.Client, name string, public bool) (map[string]any, error) {
	out := map[string]any{}
	spec := bucketSpec{ID: name, Name: name, Public: public}
	req := httpx.Request{Method: http.MethodPost, Path: bucketPath(), JSON: spec}
	if err := cl.JSON(ctx, req, &out); err != nil {
		return nil, httpx.NotEnabled("storage", err)
	}
	return out, nil
}

func newBucketsDeleteCmd(e *env.Env) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a bucket and every object it holds",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if err := checkBucket(name); err != nil {
				return err
			}
			if err := confirm(e, yes, fmt.Sprintf("delete bucket %s and all its objects?", name)); err != nil {
				return err
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			var out map[string]any
			req := httpx.Request{Method: http.MethodDelete, Path: bucketPath(name)}
			if err := cl.JSON(cmd.Context(), req, &out); err != nil {
				return err
			}
			return printResult(e, with(out, "deleted", name))
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

// checkBucket refuses a name that would not survive the request path unchanged.
func checkBucket(name string) error {
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") {
		return exitcode.Usagef("bucket name %q is invalid: one segment, no dots", name)
	}
	return nil
}

// confirm gates a destructive command: --yes skips the question, any other answer
// aborts before a single byte reaches the server.
func confirm(e *env.Env, yes bool, question string) error {
	if yes {
		return nil
	}
	ok, err := e.Confirm(question)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted: nothing was deleted")
	}
	return nil
}

// with adds k to a possibly nil map so a confirmation always names its object.
func with(m map[string]any, k string, v any) map[string]any {
	if m == nil {
		m = map[string]any{}
	}
	m[k] = v
	return m
}
