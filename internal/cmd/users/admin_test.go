package users_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/exitcode"
)

func TestDeleteUserWithYes(t *testing.T) {
	h, f := newWorld(t, "", answerEvery(200, `{"id":"u1","email":"a@b.c"}`))
	mustRun(t, h, "delete", "u1", "--yes")
	c := f.last(t)
	if c.method != http.MethodDelete || c.path != "/auth/v1/admin/users/u1" {
		t.Errorf("request = %s %s", c.method, c.path)
	}
	if !strings.Contains(h.Out.String(), "u1") {
		t.Errorf("nothing printed:\n%s", h.Out)
	}
}

func TestDeleteUserWithoutYesIsRefused(t *testing.T) {
	h, f := world(t)
	if got := exitCode(t, h, "delete", "u1"); got != exitcode.Usage {
		t.Errorf("exit %d, want 2", got)
	}
	if f.count() != 0 {
		t.Error("nothing may be deleted without --yes")
	}
}

func TestBanUserDefaultDuration(t *testing.T) {
	h, f := world(t)
	mustRun(t, h, "ban", "u1")
	c := f.last(t)
	if c.method != http.MethodPut || c.path != "/auth/v1/admin/users/u1" {
		t.Errorf("request = %s %s", c.method, c.path)
	}
	body := bodyOf(t, c)
	if len(body) != 1 || body["ban_duration"] != "876000h" {
		t.Errorf("body = %v, want only the default ban duration", body)
	}
}

func TestBanUserCustomDuration(t *testing.T) {
	h, f := world(t)
	mustRun(t, h, "ban", "u1", "--duration", "24h")
	if got := bodyOf(t, f.last(t))["ban_duration"]; got != "24h" {
		t.Errorf("ban_duration = %v, want 24h", got)
	}
}

func TestBanUserRejectsBrokenDuration(t *testing.T) {
	h, f := world(t)
	if got := exitCode(t, h, "ban", "u1", "--duration", "fortnight"); got != exitcode.Usage {
		t.Errorf("exit %d, want 2", got)
	}
	if f.count() != 0 {
		t.Error("a broken duration must not reach the server")
	}
}

func TestUnbanUser(t *testing.T) {
	h, f := world(t)
	mustRun(t, h, "unban", "u1")
	if got := bodyOf(t, f.last(t))["ban_duration"]; got != "none" {
		t.Errorf("ban_duration = %v, want none", got)
	}
}

func TestBanAndUnbanPrintTheUser(t *testing.T) {
	h, _ := world(t)
	mustRun(t, h, "ban", "u1")
	if got := outObject(t, h); got["id"] != "u1" {
		t.Errorf("printed %v", got)
	}
}

func TestRevokeSessions(t *testing.T) {
	h, f := newWorld(t, "", answerEvery(http.StatusOK, ""))
	mustRun(t, h, "sessions", "revoke", "u1")
	c := f.last(t)
	if c.method != http.MethodPost || c.path != "/auth/v1/admin/users/u1/logout" {
		t.Errorf("request = %s %s", c.method, c.path)
	}
	if !strings.Contains(h.Out.String(), "u1") {
		t.Errorf("nothing printed:\n%s", h.Out)
	}
}

func TestRevokeSessionsNotEnabledOnThisDeployment(t *testing.T) {
	h, f := newWorld(t, "", answerEvery(404, `{"msg":"no route"}`))
	err := run(t, h, "sessions", "revoke", "u1")
	if exitcode.Of(err) != exitcode.NotFound {
		t.Fatalf("exit %d, want 4: %v", exitcode.Of(err), err)
	}
	if !strings.Contains(err.Error(), "not enabled on this deployment") {
		t.Errorf("error = %v", err)
	}
	if f.count() != 1 {
		t.Errorf("%d requests, want 1", f.count())
	}
}

func TestRevokeSessionsWithYes(t *testing.T) {
	h, f := newWorld(t, "", answerEvery(http.StatusNoContent, ""))
	mustRun(t, h, "sessions", "revoke", "u1", "--yes")
	if f.count() != 1 {
		t.Errorf("%d requests, want 1", f.count())
	}
}

func TestServerErrorExitCodes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		args   []string
		want   int
	}{
		{"list is unauthorized", http.StatusUnauthorized, []string{"list"}, exitcode.AuthFail},
		{"list is forbidden", http.StatusForbidden, []string{"list"}, exitcode.AuthFail},
		{"list is missing", http.StatusNotFound, []string{"list"}, exitcode.NotFound},
		{"list breaks", http.StatusInternalServerError, []string{"list"}, exitcode.Failure},
		{"list is unavailable", http.StatusBadGateway, []string{"list"}, exitcode.Failure},
		{"create is unauthorized", http.StatusUnauthorized, []string{"create", "--email", "a@b.c"}, exitcode.AuthFail},
		{"invite is forbidden", http.StatusForbidden, []string{"invite", "--email", "a@b.c"}, exitcode.AuthFail},
		{"update breaks", http.StatusInternalServerError, []string{"update", "u1", "--role", "x"}, exitcode.Failure},
		{"delete is forbidden", http.StatusForbidden, []string{"delete", "u1", "--yes"}, exitcode.AuthFail},
		{"ban breaks", http.StatusInternalServerError, []string{"ban", "u1"}, exitcode.Failure},
		{"unban is missing", http.StatusNotFound, []string{"unban", "u1"}, exitcode.NotFound},
		{"revoke is unauthorized", http.StatusUnauthorized, []string{"sessions", "revoke", "u1"}, exitcode.AuthFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newWorld(t, "", answerEvery(tc.status, `{"msg":"boom"}`))
			if got := exitCode(t, h, tc.args...); got != tc.want {
				t.Errorf("exit %d, want %d", got, tc.want)
			}
		})
	}
}

func TestUsageErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"list takes no argument", []string{"list", "extra"}},
		{"get needs an id", []string{"get"}},
		{"get takes one argument", []string{"get", "a", "b"}},
		{"delete needs an id", []string{"delete", "--yes"}},
		{"ban needs an id", []string{"ban"}},
		{"unban needs an id", []string{"unban"}},
		{"revoke needs an id", []string{"sessions", "revoke"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, f := world(t)
			if got := exitCode(t, h, tc.args...); got != exitcode.Usage {
				t.Errorf("exit %d, want 2", got)
			}
			if f.count() != 0 {
				t.Error("a usage error must not reach the server")
			}
		})
	}
}
