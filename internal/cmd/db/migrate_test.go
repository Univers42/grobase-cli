package db_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/db"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/secret"
	"github.com/Univers42/grobase-cli/internal/testenv"
)

const serviceKey = "service-role-key-for-tests"

// fakeAdmin is the two admin routes db migrate talks to: the tenant's mount listing and
// the data plane's migrate. failOn names a migration answered with refuse.
type fakeAdmin struct {
	mounts  string
	failOn  string
	refuse  int
	tenants []string
	keys    []string
	applied []map[string]any
}

func (f *fakeAdmin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.tenants = append(f.tenants, r.Header.Get("X-Baas-Tenant-Id"))
	f.keys = append(f.keys, r.Header.Get("apikey"))
	switch r.Method + " " + r.URL.Path {
	case "GET /admin/v1/databases":
		_, _ = io.WriteString(w, f.mounts)
	case "POST /admin/v1/migrate":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] == f.failOn {
			w.WriteHeader(f.refuse)
			_, _ = io.WriteString(w, `{"error":"bad_request","message":"syntax error at or near \"CREAT\""}`)
			return
		}
		f.applied = append(f.applied, body)
		_, _ = io.WriteString(w, `{"name":"`+body["name"].(string)+`","status":"applied","statements_run":1}`)
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"no Route matched with those values"}`)
	}
}

const agencyMounts = `[{"id":"m-pg","tenant_id":"agency","name":"pg-commerce","engine":"postgresql"},` +
	`{"id":"m-my","tenant_id":"agency","name":"mysql-ops","engine":"mysql"}]`

func migrationDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, sql := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(sql), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func adminHarness(t *testing.T, f *fakeAdmin) *testenv.Harness {
	t.Helper()
	return testenv.New(t, f, map[string]string{secret.KindServiceRole: serviceKey}, "")
}

func TestMigrateAppliesADirectoryInNameOrder(t *testing.T) {
	dir := migrationDir(t, map[string]string{
		"0002_seed.sql":  "INSERT INTO notes VALUES (1, 'a');\nINSERT INTO notes VALUES (2, 'b');",
		"0001_notes.sql": "CREATE TABLE notes (id int PRIMARY KEY, body text);",
		"README.md":      "not a migration",
	})
	f := &fakeAdmin{mounts: agencyMounts}
	h := adminHarness(t, f)
	if code := run(t, h, "migrate", "--tenant", "agency", "--mount", "pg-commerce", dir); code != 0 {
		t.Fatalf("exit %d: %s", code, h.Err)
	}
	if len(f.applied) != 2 || f.applied[0]["name"] != "0001_notes" || f.applied[1]["name"] != "0002_seed" {
		t.Fatalf("applied = %v", f.applied)
	}
	seed := f.applied[1]
	if st := seed["statements"].([]any); len(st) != 1 || !strings.Contains(st[0].(string), "(2, 'b')") {
		t.Errorf("a file must travel whole as one statement: %v", st)
	}
	mount := seed["mount"].(map[string]any)
	ref := mount["credential_ref"].(map[string]any)
	identity := seed["identity"].(map[string]any)
	if mount["id"] != "m-pg" || mount["engine"] != "postgresql" || mount["tenant_id"] != "agency" ||
		ref["provider"] != "adapter-registry" || ref["reference"] != "m-pg" || mount["inline_dsn"] != nil {
		t.Errorf("mount = %v", mount)
	}
	if identity["tenant_id"] != "agency" || identity["source"] != "service_token" {
		t.Errorf("identity = %v", identity)
	}
	for i := range f.tenants {
		if f.tenants[i] != "agency" || f.keys[i] != serviceKey {
			t.Errorf("request %d: tenant %q, service key sent %v", i, f.tenants[i], f.keys[i] == serviceKey)
		}
	}
	rows, _ := decodeOut(t, h).([]any)
	if len(rows) != 2 || rows[1].(map[string]any)["status"] != "applied" {
		t.Errorf("output = %s", h.Out)
	}
}

func TestMigrateStopsAtTheFirstFailure(t *testing.T) {
	dir := migrationDir(t, map[string]string{
		"0001_a.sql": "CREATE TABLE a (id int);", "0002_b.sql": "CREAT TABLE b;", "0003_c.sql": "CREATE TABLE c (id int);",
	})
	f := &fakeAdmin{mounts: agencyMounts, failOn: "0002_b", refuse: http.StatusBadRequest}
	h := adminHarness(t, f)
	if code := run(t, h, "migrate", "--tenant", "agency", "--mount", "m-my", dir); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, h.Err)
	}
	if len(f.applied) != 1 {
		t.Fatalf("0003 ran after 0002 failed: %v", f.applied)
	}
	if !strings.Contains(h.Out.String(), "0001_a") {
		t.Errorf("what was applied before the failure must be printed: %s", h.Out)
	}
}

func TestMigrateRefusesBeforeSendingAnything(t *testing.T) {
	dup := migrationDir(t, map[string]string{"0001_a.sql": "SELECT 1;"})
	other := migrationDir(t, map[string]string{"0001_a.sql": "SELECT 2;"})
	empty := migrationDir(t, map[string]string{"0001_e.sql": " \n"})
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"no tenant", []string{"migrate", "--mount", "pg-commerce", dup}, 2},
		{"one name twice", []string{"migrate", "--tenant", "agency", "--mount", "pg-commerce", dup, other}, 2},
		{"empty file", []string{"migrate", "--tenant", "agency", "--mount", "pg-commerce", empty}, 2},
		{"missing file", []string{"migrate", "--tenant", "agency", "--mount", "pg-commerce", dup + "/nope.sql"}, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAdmin{mounts: agencyMounts}
			h := adminHarness(t, f)
			if code := run(t, h, tc.args...); code != tc.want {
				t.Fatalf("exit %d, want %d: %s", code, tc.want, h.Err)
			}
			if len(f.tenants) != 0 {
				t.Errorf("%d request(s) sent", len(f.tenants))
			}
		})
	}
}

func TestMigrateUnknownMountNamesTheTenantsMounts(t *testing.T) {
	f := &fakeAdmin{mounts: agencyMounts}
	h := adminHarness(t, f)
	err := h.Run(db.New(h.Env), "migrate", "--tenant", "agency", "--mount", "pg-comerce",
		migrationDir(t, map[string]string{"1.sql": "SELECT 1;"}))
	if code := exitcode.Of(err); code != 4 || len(f.applied) != 0 {
		t.Fatalf("exit %d, applied %d", code, len(f.applied))
	}
	if !strings.Contains(err.Error(), "mysql-ops, pg-commerce") {
		t.Errorf("the error must list the tenant's mounts: %v", err)
	}
}

func TestDBListTenantUsesTheAdminRoute(t *testing.T) {
	f := &fakeAdmin{mounts: agencyMounts}
	h := adminHarness(t, f)
	if code := run(t, h, "list", "--tenant", "agency"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.Err)
	}
	if rows, _ := decodeOut(t, h).([]any); len(rows) != 2 || f.tenants[0] != "agency" || f.keys[0] != serviceKey {
		t.Fatalf("rows %v, tenant %q", rows, f.tenants)
	}
}
