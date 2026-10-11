package storage_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/exitcode"
)

func TestObjectsUploadSendsFileAndHeaders(t *testing.T) {
	src := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(src, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := &server{body: `{"Key":"b/k.txt"}`}
	if _, err := run(t, srv, "", "cp", src, "grobase:b/k.txt", "--overwrite"); err != nil {
		t.Fatal(err)
	}
	c := srv.last(t)
	if c.Method != http.MethodPost || c.Path != "/storage/v1/object/b/k.txt" || c.Upsert != "true" ||
		c.ContentLength != 5 || !strings.HasPrefix(c.ContentType, "text/plain") {
		t.Fatalf("upload = %+v", c)
	}
}

func TestObjectsDownloadToFileAndStdout(t *testing.T) {
	srv := &server{body: "payload"}
	dst := filepath.Join(t.TempDir(), "out")
	if _, err := run(t, srv, "", "cp", "grobase:b/dir/k", dst); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "payload" {
		t.Fatalf("file = %q", got)
	}
	h, err := run(t, srv, "", "cp", "grobase:b/dir/k", "-")
	if err != nil || h.Out.String() != "payload" {
		t.Fatalf("stdout = %q, err = %v", h.Out, err)
	}
	if c := srv.last(t); c.Method != http.MethodGet || c.Path != "/storage/v1/object/b/dir/k" {
		t.Fatalf("download = %+v", c)
	}
}

func TestObjectsCpUsage(t *testing.T) {
	for _, args := range [][]string{
		{"cp", "a", "b"}, {"cp", "grobase:a/b", "grobase:c/d"}, {"cp", "a"}, {"cp", "grobase:nokey", "x"},
	} {
		srv := &server{body: "{}"}
		if code := tree(t, srv, args...); code != exitcode.Usage || srv.count() != 0 {
			t.Errorf("%v: exit %d, hits %d", args, code, srv.count())
		}
	}
}

func TestObjectsLsBodyAndSize(t *testing.T) {
	srv := &server{body: `[{"name":"k","id":"i","metadata":{"size":7}}]`}
	h, err := run(t, srv, "", "ls", "b", "p/", "--limit", "5", "--offset", "10")
	if err != nil {
		t.Fatal(err)
	}
	c := srv.last(t)
	if c.Path != "/storage/v1/object/list/b" || c.Body["prefix"] != "p/" || c.Body["limit"] != float64(5) || c.Body["offset"] != float64(10) {
		t.Fatalf("ls = %+v", c)
	}
	if !strings.Contains(h.Out.String(), `"size": 7`) && !strings.Contains(h.Out.String(), `"size":7`) {
		t.Fatalf("size not lifted: %s", h.Out)
	}
}

func TestObjectsRm(t *testing.T) {
	srv := &server{status: http.StatusNoContent}
	if _, err := run(t, srv, "", "rm", "b/k1", "b/k2", "--yes"); err != nil || srv.count() != 2 {
		t.Fatalf("hits %d, err %v", srv.count(), err)
	}
	srv = &server{status: http.StatusNoContent}
	if _, err := run(t, srv, "n\n", "rm", "b/k1"); err == nil || srv.count() != 0 {
		t.Fatalf("declined rm: hits %d, err %v", srv.count(), err)
	}
	srv = &server{status: http.StatusNoContent}
	if code := tree(t, srv, "rm", "b/k1", "bad", "--yes"); code != exitcode.Usage || srv.count() != 0 {
		t.Fatalf("a bad ref must delete nothing: exit %d hits %d", code, srv.count())
	}
}

func TestObjectsSign(t *testing.T) {
	srv := &server{body: `{"signedURL":"/storage/v1/object/sign/b/k?token=t"}`}
	h, err := run(t, srv, "", "sign", "b/k", "--expires", "60")
	if err != nil {
		t.Fatal(err)
	}
	if c := srv.last(t); c.Path != "/storage/v1/object/sign/b/k" || c.Body["expiresIn"] != float64(60) {
		t.Fatalf("sign = %+v", c)
	}
	if !strings.Contains(h.Out.String(), `"url"`) {
		t.Fatalf("no url: %s", h.Out)
	}
	if code := tree(t, &server{body: `{}`}, "sign", "b/k"); code != 1 {
		t.Fatalf("answer without a URL: exit %d", code)
	}
	if code := tree(t, &server{body: `{}`}, "sign", "b/k", "--expires", "0"); code != exitcode.Usage {
		t.Fatalf("expires 0: exit %d", code)
	}
}

func TestObjectsErrorCodes(t *testing.T) {
	t.Run("ls", func(t *testing.T) { errorTable(t, []string{"ls", "b"}, false) })
	t.Run("rm", func(t *testing.T) { errorTable(t, []string{"rm", "b/k", "--yes"}, false) })
	t.Run("sign", func(t *testing.T) { errorTable(t, []string{"sign", "b/k"}, false) })
	t.Run("cp down", func(t *testing.T) { errorTable(t, []string{"cp", "grobase:b/k", "-"}, false) })
}
