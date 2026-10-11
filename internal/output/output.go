// Package output renders command results as table, JSON or YAML, optionally
// filtered through a jq expression.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/itchyny/gojq"
	"gopkg.in/yaml.v3"
)

// Printer writes results to W.
type Printer struct {
	W      io.Writer
	Format string // table, json or yaml
	JQ     string // optional jq expression; forces JSON-lines output
}

// Print renders v. cols names the columns shown in table form for list results
// (all keys, sorted, when empty).
func (p Printer) Print(v any, cols ...string) error {
	norm, err := normalize(v)
	if err != nil {
		return err
	}
	if p.JQ != "" {
		return p.filter(norm)
	}
	switch p.Format {
	case "json":
		return writeJSON(p.W, norm)
	case "yaml":
		b, err := yaml.Marshal(norm)
		if err != nil {
			return fmt.Errorf("encode yaml: %w", err)
		}
		_, err = p.W.Write(b)
		return err
	case "table", "":
		return p.table(norm, cols)
	}
	return fmt.Errorf("unknown output format %q (want table, json or yaml)", p.Format)
}

// normalize turns any value into plain maps/slices/scalars via JSON.
func normalize(v any) (any, error) {
	if raw, ok := v.(json.RawMessage); ok {
		var out any
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("decode json: %w", err)
		}
		return out, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode value: %w", err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode value: %w", err)
	}
	return out, nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (p Printer) filter(v any) error {
	q, err := gojq.Parse(p.JQ)
	if err != nil {
		return fmt.Errorf("invalid --jq expression: %w", err)
	}
	it := q.Run(v)
	for {
		out, ok := it.Next()
		if !ok {
			return nil
		}
		if e, isErr := out.(error); isErr {
			return fmt.Errorf("--jq: %w", e)
		}
		if s, isStr := out.(string); isStr {
			_, err = fmt.Fprintln(p.W, s)
		} else {
			err = writeJSON(p.W, out)
		}
		if err != nil {
			return err
		}
	}
}

func (p Printer) table(v any, cols []string) error {
	tw := tabwriter.NewWriter(p.W, 0, 8, 2, ' ', 0)
	switch t := v.(type) {
	case []any:
		rows := toRows(t)
		if len(cols) == 0 {
			cols = keysOf(rows)
		}
		writeRow(tw, upper(cols))
		for _, r := range rows {
			writeRow(tw, cells(r, cols))
		}
	case map[string]any:
		keys := keysOf([]map[string]any{t})
		for _, k := range keys {
			writeRow(tw, []string{k, cell(t[k])})
		}
	default:
		writeRow(tw, []string{cell(v)})
	}
	return tw.Flush()
}

func toRows(list []any) []map[string]any {
	rows := make([]map[string]any, 0, len(list))
	for _, e := range list {
		if m, ok := e.(map[string]any); ok {
			rows = append(rows, m)
		} else {
			rows = append(rows, map[string]any{"value": e})
		}
	}
	return rows
}

func keysOf(rows []map[string]any) []string {
	seen := map[string]bool{}
	var keys []string
	for _, r := range rows {
		for k := range r {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

func upper(cols []string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = strings.ToUpper(c)
	}
	return out
}

func cells(r map[string]any, cols []string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = cell(r[c])
	}
	return out
}

func cell(v any) string {
	switch t := v.(type) {
	case nil:
		return "-"
	case string:
		return strings.ReplaceAll(t, "\n", " ")
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func writeRow(w io.Writer, cols []string) { _, _ = fmt.Fprintln(w, strings.Join(cols, "\t")) }
