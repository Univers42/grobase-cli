package storage_test

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/storage"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

func TestBucketsListBothShapes(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"array", `[{"id":"photos","name":"photos","public":true},{"id":"logs","name":"logs"}]`, 2},
		{"wrapped", `{"buckets":[{"id":"photos","name":"photos"}]}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := &server{body: tc.body}
			h, err := run(t, srv, "", "buckets", "list")
			if err != nil {
				t.Fatalf("exit %d: %v", exitcode.Of(err), err)
			}
			c := srv.last(t)
			if c.Method != http.MethodGet || c.Path != bucketsPath {
				t.Fatalf("request = %s %s", c.Method, c.Path)
			}
			if c.Query != "" {
				t.Fatalf("query = %q, want none", c.Query)
			}
			if c.Auth != "Bearer "+apiKey || c.APIKey != "" {
				t.Fatalf("auth = %q / %q", c.Auth, c.APIKey)
			}
			got, ok := decodeOut(t, h).([]any)
			if !ok || len(got) != tc.want {
				t.Fatalf("buckets = %v", h.Out)
			}
		})
	}
}

func TestBucketsListTableColumns(t *testing.T) {
	body := `[{"id":"photos","name":"photos","public":true,"created_at":"2026-01-02T03:04:05Z"},` +
		`{"id":"logs","name":"logs","public":false,"created_at":"2026-01-03T00:00:00Z"}]`
	h := tableHarness(t, &server{body: body})
	if err := h.Run(storage.New(h.Env), "buckets", "list"); err != nil {
		t.Fatalf("exit %d: %v", exitcode.Of(err), err)
	}
	lines := strings.Split(strings.TrimSpace(h.Out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("table = %q", h.Out)
	}
	if got, want := strings.Fields(lines[0]), strings.Join([]string{"ID", "NAME", "PUBLIC", "CREATED_AT"}, " "); strings.Join(got, " ") != want {
		t.Fatalf("header = %q, want %q", lines[0], want)
	}
	if !strings.Contains(lines[1], "photos") || !strings.Contains(lines[2], "logs") {
		t.Fatalf("rows = %q", lines[1:])
	}
}

func TestBucketsListEmpty(t *testing.T) {
	h, err := run(t, &server{body: `{"buckets":[]}`}, "", "buckets", "list")
	if err != nil {
		t.Fatalf("exit %d: %v", exitcode.Of(err), err)
	}
	if got, _ := decodeOut(t, h).([]any); len(got) != 0 {
		t.Fatalf("buckets = %v", h.Out)
	}
}

func TestBucketsCreateSendsIdNamePublic(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{"private", []string{"buckets", "create", "photos"}, false},
		{"public", []string{"buckets", "create", "photos", "--public"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := &server{body: fmt.Sprintf(`{"id":"photos","public":%t}`, tc.want)}
			h, err := run(t, srv, "", tc.args...)
			if err != nil {
				t.Fatalf("exit %d: %v", exitcode.Of(err), err)
			}
			c := srv.last(t)
			if c.Method != http.MethodPost || c.Path != bucketsPath {
				t.Fatalf("request = %s %s", c.Method, c.Path)
			}
			want := map[string]any{"id": "photos", "name": "photos", "public": tc.want}
			if !reflect.DeepEqual(c.Body, want) {
				t.Fatalf("body = %v, want %v", c.Body, want)
			}
			if got, _ := decodeOut(t, h).(map[string]any); got["id"] != "photos" {
				t.Fatalf("output = %v", got)
			}
		})
	}
}

func TestBucketsCreateRejectsBadNames(t *testing.T) {
	for _, name := range []string{"", "a/b", "..", ".", "with space/../x"} {
		srv := &server{body: `{}`}
		if code := tree(t, srv, "buckets", "create", name); code != 2 {
			t.Fatalf("name %q: exit %d, want 2", name, code)
		}
		if srv.count() != 0 {
			t.Fatalf("name %q: a request was sent", name)
		}
	}
}

func TestBucketsDelete(t *testing.T) {
	t.Run("confirmed", func(t *testing.T) {
		srv := &server{status: http.StatusNoContent}
		h := tableHarness(t, srv)
		if err := h.Run(storage.New(h.Env), "buckets", "delete", "photos", "--yes"); err != nil {
			t.Fatalf("exit %d: %v", exitcode.Of(err), err)
		}
		c := srv.last(t)
		if c.Method != http.MethodDelete || c.Path != bucketsPath+"/photos" {
			t.Fatalf("request = %s %s", c.Method, c.Path)
		}
		if c.Auth != "Bearer "+apiKey {
			t.Fatalf("auth = %q", c.Auth)
		}
		if !strings.Contains(h.Out.String(), "photos") {
			t.Fatalf("the deleted bucket is not in the output: %q", h.Out)
		}
	})

	t.Run("asks without --yes", func(t *testing.T) {
		srv := &server{status: http.StatusNoContent}
		if code := tree(t, srv, "buckets", "delete", "photos"); code != 1 {
			t.Fatalf("exit %d, want 1 (declined)", code)
		}
		if srv.count() != 0 {
			t.Fatal("DELETE was sent without confirmation")
		}
	})
}

func TestBucketsErrorCodes(t *testing.T) {
	t.Run("list", func(t *testing.T) { errorTable(t, []string{"buckets", "list"}, true) })
	t.Run("create", func(t *testing.T) { errorTable(t, []string{"buckets", "create", "photos"}, true) })
	t.Run("delete", func(t *testing.T) { errorTable(t, []string{"buckets", "delete", "photos", "--yes"}, false) })
}

func TestBucketsUsage(t *testing.T) {
	for _, args := range [][]string{
		{"buckets", "list", "extra"},
		{"buckets", "create"},
		{"buckets", "create", "a", "b"},
		{"buckets", "delete"},
	} {
		srv := &server{body: `{}`}
		if code := tree(t, srv, args...); code != 2 {
			t.Fatalf("%v: exit %d, want 2", args, code)
		}
	}
}
