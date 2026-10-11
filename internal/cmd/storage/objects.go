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

// errNoSignedURL means the sign route answered without a usable URL.
var errNoSignedURL = errors.New("the sign route returned no URL")

// listSpec is the POST /storage/v1/object/list/{bucket} body.
type listSpec struct {
	Prefix string `json:"prefix"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type lsFlags struct {
	limit, offset int
}

func newLsCmd(e *env.Env) *cobra.Command {
	var f lsFlags
	cmd := &cobra.Command{
		Use:   "ls BUCKET [PREFIX]",
		Short: "List the objects of a bucket, optionally restricted to a prefix",
		Args:  exitcode.Args(cobra.RangeArgs(1, 2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			bucket := args[0]
			prefix := ""
			if len(args) > 1 {
				prefix = args[1]
			}
			return runLs(cmd.Context(), e, bucket, prefix, f)
		},
	}
	fl := cmd.Flags()
	fl.IntVar(&f.limit, "limit", 100, "page size")
	fl.IntVar(&f.offset, "offset", 0, "first object of the page")
	return cmd
}

func runLs(ctx context.Context, e *env.Env, bucket, prefix string, f lsFlags) error {
	if err := checkBucket(bucket); err != nil {
		return err
	}
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	rows, err := listObjects(ctx, cl, bucket, prefix, f)
	if err != nil {
		return err
	}
	return e.Printer().Print(rows, "name", "id", "updated_at", "size")
}

func listObjects(ctx context.Context, cl *httpx.Client, bucket, prefix string, f lsFlags) ([]map[string]any, error) {
	var raw json.RawMessage
	spec := listSpec{Prefix: prefix, Limit: f.limit, Offset: f.offset}
	req := httpx.Request{Method: http.MethodPost, Path: objectPath("list", bucket), JSON: spec}
	if err := cl.JSON(ctx, req, &raw); err != nil {
		return nil, err
	}
	rows, err := decodeRows(raw, "objects")
	if err != nil {
		return nil, err
	}
	for i, r := range rows {
		rows[i] = objectRow(r)
	}
	return rows, nil
}

// objectRow lifts metadata.size into a size column; the rest of the object is
// passed through untouched.
func objectRow(m map[string]any) map[string]any {
	meta, ok := m["metadata"].(map[string]any)
	if !ok {
		return m
	}
	if _, ok := meta["size"]; !ok {
		return m
	}
	row := make(map[string]any, len(m)+1)
	for k, v := range m {
		row[k] = v
	}
	row["size"] = meta["size"]
	return row
}

type rmFlags struct{ yes bool }

func newRmCmd(e *env.Env) *cobra.Command {
	var f rmFlags
	cmd := &cobra.Command{
		Use:   "rm BUCKET/KEY...",
		Short: "Delete objects from a bucket",
		Args:  exitcode.Args(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs, err := parseRefs(args)
			if err != nil {
				return err
			}
			if err := confirmRm(e, f.yes, refs); err != nil {
				return err
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			if err := removeObjects(cmd.Context(), cl, refs); err != nil {
				return err
			}
			return e.Printer().Print(map[string]any{"deleted": args})
		},
	}
	cmd.Flags().BoolVar(&f.yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

// parseRefs checks every argument up front, so a typo halfway through a list
// deletes nothing at all.
func parseRefs(args []string) ([]ref, error) {
	refs := make([]ref, 0, len(args))
	for _, a := range args {
		bucket, key, err := splitRef(a)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref{bucket: bucket, key: key})
	}
	return refs, nil
}

// confirmRm asks once for the whole batch: asking per key would be unusable on a
// long argument list, and naming the first three keeps the question readable.
func confirmRm(e *env.Env, yes bool, refs []ref) error {
	if yes {
		return nil
	}
	q := fmt.Sprintf("delete %d objects?", len(refs))
	if len(refs) <= 3 {
		q = fmt.Sprintf("delete %s?", strings.Join(refStrings(refs), ", "))
	}
	return confirm(e, yes, q)
}

func refStrings(refs []ref) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.bucket + "/" + r.key
	}
	return out
}

func removeObjects(ctx context.Context, cl *httpx.Client, refs []ref) error {
	for _, r := range refs {
		req := httpx.Request{Method: http.MethodDelete, Path: objectPath(r.bucket, r.key)}
		if err := cl.JSON(ctx, req, nil); err != nil {
			return err
		}
	}
	return nil
}

type signFlags struct{ expires int }

func newSignCmd(e *env.Env) *cobra.Command {
	var f signFlags
	cmd := &cobra.Command{
		Use:   "sign BUCKET/KEY",
		Short: "Create a time-limited URL that serves one object",
		Args:  exitcode.Args(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			bucket, key, err := splitRef(args[0])
			if err != nil {
				return err
			}
			if f.expires <= 0 {
				return exitcode.Usagef("--expires must be a positive number of seconds")
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			out, err := signObject(cmd.Context(), cl, bucket, key, f.expires)
			if err != nil {
				return err
			}
			return e.Printer().Print(out)
		},
	}
	cmd.Flags().IntVar(&f.expires, "expires", 3600, "lifetime of the URL in seconds")
	return cmd
}

// signBody is the POST /storage/v1/object/sign/{bucket}/{key} body.
type signBody struct {
	ExpiresIn int `json:"expiresIn"`
}

func signObject(ctx context.Context, cl *httpx.Client, bucket, key string, expires int) (map[string]any, error) {
	out := map[string]any{}
	req := httpx.Request{
		Method: http.MethodPost, Path: objectPath("sign", bucket, key),
		JSON: signBody{ExpiresIn: expires},
	}
	if err := cl.JSON(ctx, req, &out); err != nil {
		return nil, err
	}
	return out, normalizeSignedURL(out)
}

// normalizeSignedURL renames whichever field the deployment used to a plain "url";
// the answer is useless without it.
func normalizeSignedURL(m map[string]any) error {
	for _, k := range []string{"signedURL", "signedUrl", "signed_url", "url"} {
		if u, ok := m[k].(string); ok && u != "" {
			delete(m, k)
			m["url"] = u
			return nil
		}
	}
	return errNoSignedURL
}
