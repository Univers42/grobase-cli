// Package storage implements the `grobase storage` command.
package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// New returns the `storage` command.
func New(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "storage",
		Short: "Manage the buckets and objects of the current tenant",
		Long: "An object is addressed as BUCKET/KEY. A key may contain slashes; the remote\n" +
			"side of `storage cp` is written grobase:BUCKET/KEY.",
	}
	c.AddCommand(newBucketsCmd(e), newLsCmd(e), newCpCmd(e), newRmCmd(e), newSignCmd(e))
	return c
}

// bucketPath builds a path under /storage/v1/bucket.
func bucketPath(parts ...string) string {
	var b strings.Builder
	b.WriteString("/storage/v1/bucket")
	for _, p := range parts {
		b.WriteString("/" + httpx.Seg(p))
	}
	return b.String()
}

// objectPath builds a path under /storage/v1/object. Every segment is escaped on
// its own: an element holding slashes is split first, so the key's slashes stay
// separators while no part can smuggle an encoded slash or a traversal.
func objectPath(parts ...string) string {
	var b strings.Builder
	b.WriteString("/storage/v1/object")
	for _, p := range parts {
		for _, seg := range strings.Split(p, "/") {
			b.WriteString("/" + httpx.Seg(seg))
		}
	}
	return b.String()
}

// splitRef splits BUCKET/KEY at the first slash. The key keeps its inner slashes;
// empty and ".." segments are refused because they would change which object the
// request names.
func splitRef(s string) (bucket, key string, err error) {
	i := strings.Index(s, "/")
	if i <= 0 || i == len(s)-1 {
		return "", "", exitcode.Usagef("%q is not BUCKET/KEY", s)
	}
	bucket, key = s[:i], s[i+1:]
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || seg == ".." {
			return "", "", exitcode.Usagef("key %q has an empty or %q segment", key, "..")
		}
	}
	return bucket, key, nil
}

// printResult shows the server's answer, or a bare confirmation when the route
// replied with an empty body, so a successful command is never silent.
func printResult(e *env.Env, out map[string]any) error {
	if len(out) > 0 {
		return e.Printer().Print(out)
	}
	return e.Printer().Print(map[string]any{"ok": true})
}

// decodeRows accepts both list shapes grobase uses: a bare JSON array or an
// object wrapping it under key.
func decodeRows(raw json.RawMessage, key string) ([]map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return []map[string]any{}, nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err == nil {
		return nonNilRows(rows), nil
	}
	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("decode list: %w", err)
	}
	inner, ok := wrapped[key]
	if !ok {
		return []map[string]any{}, nil
	}
	if err := json.Unmarshal(inner, &rows); err != nil {
		return nil, fmt.Errorf("decode %s: %w", key, err)
	}
	return nonNilRows(rows), nil
}

func nonNilRows(rows []map[string]any) []map[string]any {
	if rows == nil {
		return []map[string]any{}
	}
	return rows
}
