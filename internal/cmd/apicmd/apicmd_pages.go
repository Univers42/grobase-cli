package apicmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// page is one fetched answer of the walk.
type page struct {
	status int
	header http.Header
	body   []byte
}

// target is the next request: this deployment's path plus its query.
type target struct {
	path  string
	query url.Values
}

// walk fetches the first answer and, with --paginate, every following one.
func walk(ctx context.Context, cl *httpx.Client, e *env.Env, req httpx.Request, f apiFlags) ([]page, error) {
	pages := make([]page, 0, 1)
	for {
		p, err := fetch(ctx, cl, e, req, f.include)
		if err != nil {
			return nil, err
		}
		pages = append(pages, p)
		next, more := f.nextPage(req, p)
		if !more || len(pages) >= f.maxPages {
			if more {
				e.Warnf("stopped after %d pages; raise --max-pages for the rest", len(pages))
			}
			return pages, nil
		}
		req.Path, req.Query = next.path, next.query
	}
}

// fetch sends one request, echoes the head to stderr when asked, and buffers the
// body (the walk cannot stream pages while deciding what comes next).
func fetch(ctx context.Context, cl *httpx.Client, e *env.Env, req httpx.Request, include bool) (page, error) {
	resp, err := cl.Do(ctx, req)
	if err != nil {
		return page{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return page{}, fmt.Errorf("read response: %w", err)
	}
	if include {
		printHead(e.Stderr, resp)
	}
	return page{status: resp.StatusCode, header: resp.Header, body: body}, nil
}

// printHead writes the status line and headers like curl -i, on stderr so a
// piped body stays clean.
func printHead(w io.Writer, resp *http.Response) {
	_, _ = fmt.Fprintf(w, "%s %s\n", resp.Proto, resp.Status)
	for name, values := range resp.Header {
		for _, v := range values {
			_, _ = fmt.Fprintf(w, "%s: %s\n", name, v)
		}
	}
	_, _ = fmt.Fprintln(w)
}

// nextPage decides where the walk continues, if anywhere.
func (f apiFlags) nextPage(req httpx.Request, p page) (target, bool) {
	if !f.paginate {
		return target{}, false
	}
	if link := nextLink(p.header.Get("Link")); link != "" {
		path, query, err := splitPath(link)
		if err != nil {
			return target{}, false
		}
		return target{path: path, query: query}, true
	}
	// No Link: continue on a PostgREST Content-Range answer, whose pages are
	// always arrays.
	if !isArray(p.body) {
		return target{}, false
	}
	offset, size, ok := contentRangeOffset(p.header.Get("Content-Range"))
	if !ok {
		return target{}, false
	}
	query := maps.Clone(req.Query)
	query.Set("offset", strconv.Itoa(offset))
	query.Set("limit", strconv.Itoa(size))
	return target{path: req.Path, query: query}, true
}

// nextLink returns the URL of the rel="next" entry of a Link header.
func nextLink(header string) string {
	for _, entry := range strings.Split(header, ",") {
		url, params, ok := strings.Cut(entry, ";")
		if !ok || !strings.Contains(params, `rel="next"`) {
			continue
		}
		if u := strings.Trim(strings.TrimSpace(url), "<>"); u != "" {
			return u
		}
	}
	return ""
}

// contentRangeOffset reads a Content-Range header ("0-24/100") and gives the
// offset and size of the next page, unless this one is already the last.
// Ponytail: a wildcard total ("0-24/*") or a body that is not an array stops the
// walk; only the caller can tell those apart from a real end of results.
func contentRangeOffset(header string) (offset, size int, ok bool) {
	seen, total, found := strings.Cut(header, "/")
	if !found {
		return 0, 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(total))
	if err != nil || n <= 0 {
		return 0, 0, false
	}
	first, last, found := strings.Cut(strings.TrimSpace(seen), "-")
	if !found {
		return 0, 0, false
	}
	lo, errLo := strconv.Atoi(first)
	hi, errHi := strconv.Atoi(last)
	switch {
	case errLo != nil, errHi != nil, hi+1 >= n:
		return 0, 0, false
	}
	return hi + 1, hi - lo + 1, true
}

// emit prints the pages: --raw writes the bodies exactly as received, array
// pages merge into one result, everything else prints page by page.
func emit(e *env.Env, pages []page, f apiFlags) error {
	if f.raw {
		for _, p := range pages {
			if len(bytes.TrimSpace(p.body)) == 0 {
				continue
			}
			if _, err := e.Stdout.Write(p.body); err != nil {
				return err
			}
		}
		return nil
	}
	if len(pages) > 1 {
		if merged, ok := mergeArrays(pages); ok {
			return e.Printer().Print(merged)
		}
	}
	for _, p := range pages {
		if err := writeBody(e, p.body); err != nil {
			return err
		}
	}
	return nil
}

// mergeArrays folds several array pages into one document.
func mergeArrays(pages []page) ([]any, bool) {
	out := []any{}
	for _, p := range pages {
		var rows []any
		if !isArray(p.body) || json.Unmarshal(p.body, &rows) != nil {
			return nil, false
		}
		out = append(out, rows...)
	}
	return out, true
}

// writeBody prints one answer: the printer renders JSON, anything else is text.
func writeBody(e *env.Env, body []byte) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	if !json.Valid(bytes.TrimSpace(body)) {
		_, err := fmt.Fprintln(e.Stdout, strings.TrimRight(string(body), "\r\n"))
		return err
	}
	return e.Printer().Print(json.RawMessage(body))
}

// isArray reports whether a body is a JSON array, the only shape a
// Content-Range answer can have.
func isArray(body []byte) bool {
	t := bytes.TrimSpace(body)
	return len(t) > 0 && t[0] == '['
}
