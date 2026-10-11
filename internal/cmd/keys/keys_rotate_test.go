package keys_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/exitcode"
)

const rotateList = `{"keys":[{"id":"old-1","name":"prod","scopes":["read","write"]}]}`

func TestKeysRotateSuccess(t *testing.T) {
	var methods []string
	var createBody map[string]any
	var deletePath string
	env, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch {
		case r.Method == "GET" && r.URL.Path == keysURL:
			_, _ = io.WriteString(rw, rotateList)
		case r.Method == "POST" && r.URL.Path == keysURL:
			createBody = bodyJSON(t, r)
			_, _ = io.WriteString(rw, `{"id":"new-1","name":"prod","key":"`+newKey+`"}`)
		case r.Method == "DELETE":
			deletePath = r.URL.Path
			rw.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}), "rotate", "old-1", "--yes")
	if err != nil {
		t.Fatalf("exit %d: %v", exitcode.Of(err), err)
	}
	if strings.Join(methods, ",") != "GET,POST,DELETE" {
		t.Fatalf("request order = %v", methods)
	}
	if deletePath != keysURL+"/old-1" {
		t.Fatalf("delete path = %q", deletePath)
	}
	got, _ := json.Marshal(createBody)
	if string(got) != `{"name":"prod","scopes":["read","write"]}` {
		t.Fatalf("recreate body = %s (must reuse the old name and scopes)", got)
	}
	if !strings.Contains(env.Out.String(), newKey) {
		t.Fatalf("new key not printed: %s", env.Out)
	}
	if !strings.Contains(env.Err.String(), "store this key now") {
		t.Fatalf("warning missing: %s", env.Err)
	}
}

func TestKeysRotateCreateFailureKeepsOldKey(t *testing.T) {
	deleted := false
	_, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "DELETE":
			deleted = true
		case "POST":
			rw.WriteHeader(http.StatusInternalServerError)
			return
		default: // GET list
			_, _ = io.WriteString(rw, rotateList)
		}
	}), "rotate", "old-1", "--yes")
	if got := exitcode.Of(err); got != 1 {
		t.Fatalf("exit %d, want 1 (err=%v)", got, err)
	}
	if deleted {
		t.Fatal("the old key was revoked even though the replacement failed to be created")
	}
}

func TestKeysRotateDeleteFailureSurfaces(t *testing.T) {
	env, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			_, _ = io.WriteString(rw, rotateList)
		case "POST":
			_, _ = io.WriteString(rw, `{"id":"new-1","name":"prod","key":"`+newKey+`"}`)
		case "DELETE":
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}), "rotate", "old-1", "--yes")
	if got := exitcode.Of(err); got != 1 {
		t.Fatalf("exit %d, want 1 (err=%v)", got, err)
	}
	if !strings.Contains(env.Out.String(), newKey) {
		t.Fatalf("the new key must be printed before the delete is attempted: %s", env.Out)
	}
}

func TestKeysRotateLookupErrors(t *testing.T) {
	t.Run("list 404 means not enabled", func(t *testing.T) {
		_, err := run(t, statusHandler(404), "rotate", "old-1", "--yes")
		if got := exitcode.Of(err); got != 4 {
			t.Fatalf("exit %d, want 4 (err=%v)", got, err)
		}
		if err == nil || !strings.Contains(err.Error(), "not enabled on this deployment:") {
			t.Fatalf("want not-enabled, got %v", err)
		}
	})
	t.Run("id missing from list is not found", func(t *testing.T) {
		_, err := run(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(rw, `{"keys":[{"id":"other"}]}`)
		}), "rotate", "ghost", "--yes")
		if got := exitcode.Of(err); got != 4 {
			t.Fatalf("exit %d, want 4 (err=%v)", got, err)
		}
		if err != nil && strings.Contains(err.Error(), "not enabled on this deployment:") {
			t.Fatalf("a missing id must not be relabelled not-enabled: %v", err)
		}
	})
}
