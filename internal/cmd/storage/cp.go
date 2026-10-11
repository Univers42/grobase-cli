package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// remotePrefix marks the remote side of a copy.
const remotePrefix = "grobase:"

// stdout is the local side that means "write the object to standard output".
const stdout = "-"

type cpFlags struct {
	contentType string
	overwrite   bool
}

// ref is one side of `storage cp`: a local path or a remote object.
type ref struct {
	remote      bool
	bucket, key string // remote only
	path        string // local only
}

// String puts the side back into the form the user typed.
func (r ref) String() string {
	if r.remote {
		return remotePrefix + r.bucket + "/" + r.key
	}
	return r.path
}

func newCpCmd(e *env.Env) *cobra.Command {
	var f cpFlags
	cmd := &cobra.Command{
		Use:   "cp SRC DST",
		Short: "Copy an object between the local filesystem and a bucket",
		Long: "One side is a local path, the other grobase:BUCKET/KEY. Use - as the local\n" +
			"destination to write the object to standard output.",
		Args: exitcode.Args(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := parseSide(args[0])
			if err != nil {
				return err
			}
			dst, err := parseSide(args[1])
			if err != nil {
				return err
			}
			if src.remote == dst.remote {
				side := "local"
				if src.remote {
					side = "remote"
				}
				return exitcode.Usagef("both sides are %s: one must be a path, the other %sBUCKET/KEY", side, remotePrefix)
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			if !src.remote {
				return upload(cmd.Context(), e, cl, src, dst, f)
			}
			return download(cmd.Context(), e, cl, src, dst)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.contentType, "content-type", "", "Content-Type of an upload (default: guessed from the file name)")
	fl.BoolVar(&f.overwrite, "overwrite", false, "replace an object that already exists")
	return cmd
}

// parseSide reads one side of a copy: the grobase: prefix makes it remote,
// anything else is a local path.
func parseSide(s string) (ref, error) {
	if !strings.HasPrefix(s, remotePrefix) {
		return ref{path: s}, nil
	}
	bucket, key, err := splitRef(strings.TrimPrefix(s, remotePrefix))
	if err != nil {
		return ref{}, err
	}
	return ref{remote: true, bucket: bucket, key: key}, nil
}

// upload streams the file to the object API. The body is the open file itself so
// a large object never sits in memory, and ContentLength tells the server how
// much to expect.
func upload(ctx context.Context, e *env.Env, cl *httpx.Client, src, dst ref, f cpFlags) error {
	fh, err := os.Open(src.path)
	if err != nil {
		return err
	}
	defer func() { _ = fh.Close() }()
	st, err := fh.Stat()
	if err != nil {
		return err
	}
	if st.IsDir() {
		return exitcode.Usagef("%s is a directory", src.path)
	}
	h := http.Header{}
	if f.overwrite {
		h.Set("x-upsert", "true")
	}
	req := httpx.Request{
		Method: http.MethodPost, Path: objectPath(dst.bucket, dst.key),
		Body: fh, ContentLength: st.Size(),
		ContentType: contentTypeOf(f.contentType, src.path), Header: h,
	}
	out := map[string]any{}
	if err := cl.JSON(ctx, req, &out); err != nil {
		return err
	}
	return printResult(e, merge(out, dst.String(), st.Size()))
}

// contentTypeOf guesses the Content-Type of an upload from its name, so a browser
// shows the object instead of downloading it.
func contentTypeOf(flag, path string) string {
	if flag != "" {
		return flag
	}
	return mime.TypeByExtension(filepath.Ext(path))
}

// merge puts the object we just wrote under the server's own answer.
func merge(out map[string]any, object string, size int64) map[string]any {
	res := map[string]any{}
	for k, v := range out {
		res[k] = v
	}
	res["uploaded"] = object
	res["bytes"] = size
	return res
}

// download copies the answer into a local file, or to stdout.
func download(ctx context.Context, e *env.Env, cl *httpx.Client, src, dst ref) error {
	req := httpx.Request{Method: http.MethodGet, Path: objectPath(src.bucket, src.key)}
	resp, err := cl.Do(ctx, req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if dst.path == stdout {
		if _, err := io.Copy(e.Stdout, resp.Body); err != nil {
			return fmt.Errorf("write object to stdout: %w", err)
		}
		return nil
	}
	return saveTo(dst.path, resp.Body)
}

// saveTo streams body into path through a temp file in the same directory and
// renames it once the transfer is complete: a download that dies half-way leaves
// no partial object behind.
func saveTo(path string, body io.Reader) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".grobase-download-*")
	if err != nil {
		return fmt.Errorf("create temp file next to %s: %w", path, err)
	}
	tmp := f.Name()
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()
	if _, err = io.Copy(f, body); err != nil {
		return fmt.Errorf("download %s: %w", path, err)
	}
	if err = f.Sync(); err != nil {
		return fmt.Errorf("flush %s: %w", path, err)
	}
	if err = os.Rename(tmp, path); err != nil {
		return fmt.Errorf("move download into place: %w", err)
	}
	return nil
}
