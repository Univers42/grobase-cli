package users_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/exitcode"
)

func TestCreateUserSendsTheWholeBody(t *testing.T) {
	h, f := newWorld(t, strongPassword+"\n", answerEvery(200, userJSON))
	mustRun(t, h, "create",
		"--email", "a@b.c", "--password-stdin", "--role", "admin", "--confirm",
		"--metadata", `{"team":"core"}`)
	c := f.last(t)
	if c.method != http.MethodPost || c.path != "/auth/v1/admin/users" {
		t.Errorf("request = %s %s", c.method, c.path)
	}
	body := bodyOf(t, c)
	if body["email"] != "a@b.c" || body["email_confirm"] != true {
		t.Errorf("body = %v", body)
	}
	if body["role"] != "admin" {
		t.Errorf("role = %v", body["role"])
	}
	meta, _ := body["user_metadata"].(map[string]any)
	if meta["team"] != "core" {
		t.Errorf("user_metadata = %v", body["user_metadata"])
	}
	if body["password"] != strongPassword {
		t.Error("the password was not sent with the request")
	}
	if len(body) != 5 {
		t.Errorf("body carries %d keys, want 5: %v", len(body), body)
	}
	if strings.Contains(h.Out.String()+h.Err.String(), strongPassword) {
		t.Error("the password leaked into the output")
	}
}

func TestCreateUserWithoutPassword(t *testing.T) {
	h, f := world(t)
	mustRun(t, h, "create", "--email", "a@b.c")
	body := bodyOf(t, f.last(t))
	if _, ok := body["password"]; ok {
		t.Error("no password was asked for, so none may be sent")
	}
	if _, ok := body["role"]; ok {
		t.Error("role was not requested")
	}
	if _, ok := body["user_metadata"]; ok {
		t.Error("metadata was not requested")
	}
	if body["email_confirm"] != false {
		t.Errorf("email_confirm = %v, want false", body["email_confirm"])
	}
	if len(body) != 2 {
		t.Errorf("body carries %d keys, want 2: %v", len(body), body)
	}
}

func TestCreateUserRejectsWeakPasswordsBeforeAnyRequest(t *testing.T) {
	for _, pw := range []string{"short", "alllowercaseletters", "Abcdefghijk"} {
		h, f := newWorld(t, pw+"\n", answerEvery(200, userJSON))
		if got := exitCode(t, h, "create", "--email", "a@b.c", "--password-stdin"); got != exitcode.Usage {
			t.Errorf("password %q: exit %d, want 2", pw, got)
		}
		if f.count() != 0 {
			t.Errorf("password %q: a request reached the server", pw)
		}
	}
}

func TestCreateUserRequiresAnEmail(t *testing.T) {
	h, f := world(t)
	if got := exitCode(t, h, "create"); got != exitcode.Usage {
		t.Errorf("exit %d, want 2", got)
	}
	if f.count() != 0 {
		t.Error("no request should have been sent")
	}
}

func TestCreateUserRejectsBrokenMetadata(t *testing.T) {
	h, f := world(t)
	if got := exitCode(t, h, "create", "--email", "a@b.c", "--metadata", "not json"); got != exitcode.Usage {
		t.Errorf("exit %d, want 2", got)
	}
	if got := exitCode(t, h, "create", "--email", "a@b.c", "--metadata", "[1,2]"); got != exitcode.Usage {
		t.Errorf("a JSON array is not a metadata object: exit %d, want 2", got)
	}
	if f.count() != 0 {
		t.Error("no request should have been sent")
	}
}

func TestInviteUser(t *testing.T) {
	h, f := world(t)
	mustRun(t, h, "invite", "--email", "a@b.c")
	c := f.last(t)
	if c.method != http.MethodPost || c.path != "/auth/v1/invite" {
		t.Errorf("request = %s %s", c.method, c.path)
	}
	if body := bodyOf(t, c); len(body) != 1 || body["email"] != "a@b.c" {
		t.Errorf("body = %v", body)
	}
}

func TestInviteUserRequiresAnEmail(t *testing.T) {
	h, f := world(t)
	if got := exitCode(t, h, "invite"); got != exitcode.Usage {
		t.Errorf("exit %d, want 2", got)
	}
	if f.count() != 0 {
		t.Error("no request should have been sent")
	}
}

func TestUpdateUserSendsOnlyChangedFields(t *testing.T) {
	h, f := world(t)
	mustRun(t, h, "update", "u1", "--role", "editor")
	c := f.last(t)
	if c.method != http.MethodPut || c.path != "/auth/v1/admin/users/u1" {
		t.Errorf("request = %s %s", c.method, c.path)
	}
	body := bodyOf(t, c)
	if len(body) != 1 || body["role"] != "editor" {
		t.Errorf("body = %v, want only the role", body)
	}
}

func TestUpdateUserEveryField(t *testing.T) {
	h, f := newWorld(t, strongPassword+"\n", answerEvery(200, userJSON))
	mustRun(t, h, "update", "u1",
		"--email", "new@b.c", "--role", "admin", "--password-stdin", "--metadata", `{"team":"core"}`)
	body := bodyOf(t, f.last(t))
	if len(body) != 4 {
		t.Fatalf("body carries %d keys, want 4: %v", len(body), body)
	}
	if body["email"] != "new@b.c" || body["role"] != "admin" {
		t.Errorf("body = %v", body)
	}
	if body["password"] != strongPassword {
		t.Error("the new password was not sent with the request")
	}
	meta, _ := body["user_metadata"].(map[string]any)
	if meta["team"] != "core" {
		t.Errorf("user_metadata = %v", body["user_metadata"])
	}
	if strings.Contains(h.Out.String()+h.Err.String(), strongPassword) {
		t.Error("the password leaked into the output")
	}
}

func TestUpdateUserWithoutChanges(t *testing.T) {
	h, f := world(t)
	if got := exitCode(t, h, "update", "u1"); got != exitcode.Usage {
		t.Errorf("exit %d, want 2", got)
	}
	if f.count() != 0 {
		t.Error("an empty update must not reach the server")
	}
}

func TestUpdateUserRejectsWeakPasswordBeforeAnyRequest(t *testing.T) {
	h, f := newWorld(t, "weak\n", answerEvery(200, userJSON))
	if got := exitCode(t, h, "update", "u1", "--password-stdin"); got != exitcode.Usage {
		t.Errorf("exit %d, want 2", got)
	}
	if f.count() != 0 {
		t.Error("a request reached the server")
	}
}

func TestUpdateUserByEmailResolvesTheID(t *testing.T) {
	h, f := newWorld(t, "", answerEvery(200, `{"users":[`+userJSON+`],"total":1}`))
	mustRun(t, h, "update", "a@b.c", "--role", "editor")
	if c := f.last(t); c.method != http.MethodPut || c.path != "/auth/v1/admin/users/u1" {
		t.Errorf("request = %s %s, want PUT on the resolved id", c.method, c.path)
	}
}

func TestUpdateUserUnknownEmailSendsNoWrite(t *testing.T) {
	h, f := newWorld(t, "", answerEvery(200, `{"users":[],"total":0}`))
	if got := exitCode(t, h, "update", "nobody@b.c", "--role", "editor"); got != exitcode.NotFound {
		t.Errorf("exit %d, want 4", got)
	}
	if c := f.last(t); c.method != http.MethodGet {
		t.Errorf("last request = %s %s, want only the lookup", c.method, c.path)
	}
}

func TestUpdateUserEscapesTheID(t *testing.T) {
	h, f := world(t)
	mustRun(t, h, "update", "u/1", "--role", "editor")
	if got := f.last(t).path; got != "/auth/v1/admin/users/u%2F1" {
		t.Errorf("path = %q, want the id escaped", got)
	}
}
