package db

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// migratePath is the data plane's admin migrate route (Kong: service-role key, acl
// baas-admin, private source ranges, 256 KB bodies).
const migratePath = "/admin/v1/migrate"

// migrationUser names the CLI in the identity a migration is applied under.
const migrationUser = "grobase-cli"

// migration is one .sql file, read whole.
type migration struct{ name, path, sql string }

func newMigrateCmd(e *env.Env) *cobra.Command {
	var tenant, mount string
	cmd := &cobra.Command{
		Use:   "migrate --tenant <id> --mount <name|id> <file.sql|dir>...",
		Short: "Apply SQL migrations to one tenant's database mount (service-role key)",
		Long: "Each .sql file is one migration, named by its file name without .sql; a directory\n" +
			"gives its *.sql files in name order (prefix them 0001_, 0002_, ...). A file is sent\n" +
			"whole, so it may hold several statements: Postgres applies it in one transaction,\n" +
			"MySQL statement by statement (its DDL commits on its own). The server records each\n" +
			"name in the database's _baas_migrations table and skips a name it already holds, so\n" +
			"a re-run is safe and a changed file needs a new name. The run stops at the first\n" +
			"failure, since later files usually build on earlier ones.\n\n" +
			"Engines that advertise ddl apply migrations (postgresql, mysql); the others are\n" +
			"refused. The server resolves the mount's credentials itself, so no connection\n" +
			"string is read or sent.",
		Args: exitcode.Args(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrate(cmd.Context(), e, tenant, mount, args)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&tenant, "tenant", "", "tenant that owns the mount (required)")
	fl.StringVar(&mount, "mount", "", "mount name or id (required; see `grobase db list --tenant`)")
	return cmd
}

func runMigrate(ctx context.Context, e *env.Env, tenant, mountRef string, args []string) error {
	tenant, mountRef = strings.TrimSpace(tenant), strings.TrimSpace(mountRef)
	if tenant == "" || mountRef == "" {
		return exitcode.Usagef("--tenant and --mount are required")
	}
	migrations, err := readMigrations(args)
	if err != nil {
		return err
	}
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	rows, err := tenantMounts(ctx, cl, tenant)
	if err != nil {
		return err
	}
	mount, err := findMount(rows, mountRef)
	if err != nil {
		return err
	}
	results := make([]map[string]any, 0, len(migrations))
	for _, m := range migrations {
		var out map[string]any
		req := httpx.TenantRequest(http.MethodPost, migratePath, tenant, migrateBody(tenant, mount, m))
		if err := cl.JSON(ctx, req, &out); err != nil {
			if len(results) > 0 {
				_ = printMigrations(e, results)
			}
			return fmt.Errorf("migration %s (%s): %w", m.name, m.path, httpx.NotEnabled("migrations", err))
		}
		results = append(results, map[string]any{
			"name": m.name, "status": out["status"], "statements_run": out["statements_run"], "file": m.path,
		})
	}
	return printMigrations(e, results)
}

func printMigrations(e *env.Env, results []map[string]any) error {
	return e.Printer().Print(results, "name", "status", "statements_run", "file")
}

// readMigrations expands the arguments into migrations, every file read before anything
// is sent: a file as given, a directory's *.sql in name order. Two files with one name
// would make the second a silent skip, and an empty file would record a name that did
// nothing, so both are refused.
func readMigrations(args []string) ([]migration, error) {
	var paths []string
	for _, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			paths = append(paths, a)
			continue
		}
		found, err := filepath.Glob(filepath.Join(a, "*.sql"))
		if err != nil {
			return nil, err
		}
		if len(found) == 0 {
			return nil, exitcode.Usagef("%s holds no .sql file", a)
		}
		sort.Strings(found)
		paths = append(paths, found...)
	}
	seen := make(map[string]string, len(paths))
	out := make([]migration, 0, len(paths))
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".sql")
		if prev, ok := seen[name]; ok {
			return nil, exitcode.Usagef("%s and %s are both migration %q: the second would be skipped", prev, p, name)
		}
		seen[name] = p
		b, err := os.ReadFile(p) //nolint:gosec // G304: the file is chosen by the operator, like a shell redirect
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(string(b)) == "" {
			return nil, exitcode.Usagef("%s is empty", p)
		}
		out = append(out, migration{name: name, path: p, sql: string(b)})
	}
	return out, nil
}

// migrateBody is the data plane's AdminMigrateRequest. The mount carries no secret: the
// data plane fetches the DSN from the adapter-registry by mount id, scoped to the tenant,
// so another tenant's id resolves to nothing.
func migrateBody(tenant string, mount map[string]any, m migration) map[string]any {
	id := field(mount, "id")
	return map[string]any{
		"identity": map[string]any{
			"tenant_id": tenant, "user_id": migrationUser, "source": "service_token", "roles": []string{"service_role"},
		},
		"mount": map[string]any{
			"id": id, "tenant_id": tenant, "engine": field(mount, "engine"), "name": field(mount, "name"),
			"credential_ref": map[string]any{"provider": "adapter-registry", "reference": id, "version": "1"},
		},
		"name":       m.name,
		"statements": []string{m.sql},
	}
}
