// Package query implements the `grobase query` command: raw SQL, row CRUD and
// schema discovery against the query service of the current context.
package query

import (
	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// Routes of the query service.
const (
	execPath    = "/query/v1/execute"
	enginesPath = "/query/v1/engines"
)

// New returns the `query` command.
func New(e *env.Env) *cobra.Command {
	c := &cobra.Command{
		Use:   "query",
		Short: "Run SQL and row CRUD against a mounted database",
		Long: "Talk to the query service of the current context:\n" +
			"  run      run one read-only SQL statement (inline, from a file or from stdin)\n" +
			"  crud     select/insert/update/delete rows of a table\n" +
			"  engines  list the database engines the service supports\n" +
			"  schema   show the tables and columns of a database",
	}
	// Every subcommand authenticates the same way, so --mode lives on the parent.
	mode := c.PersistentFlags().String("mode", "auto", "credential to send: auto (a stored API key before the session), user, service, apikey, anon or none")
	c.AddCommand(newRunCmd(e, mode), newCrudCmd(e, mode), newEnginesCmd(e, mode), newSchemaCmd(e, mode))
	return c
}

func newRunCmd(e *env.Env, mode *string) *cobra.Command {
	var f runFlags
	cmd := &cobra.Command{
		Use:   "run --db ID (--sql 'SELECT 1' | --file FILE | --file -)",
		Short: "Run one read-only SQL statement against a database",
		Long: "Send one statement to the query service's SQL runner (POST /query/v1/{db}/sql-ro).\n" +
			"The server decides what runs: one read-only statement, on a postgres mount\n" +
			"that is not owner-scoped, with QUERY_ROUTER_SQL_RO on. Anything else is a\n" +
			"400 naming the rule, or a bare 404 that never says whether the mount exists.\n" +
			"Other engines and owner-scoped mounts: `grobase query crud`.",
		Args: exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if f.db == "" {
				return exitcode.Usagef("--db is required")
			}
			m, err := parseMode(*mode)
			if err != nil {
				return err
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			return runSQL(cmd.Context(), e, cl, f, m)
		},
	}
	cmd.Flags().StringVar(&f.db, "db", "", "database id to run the statement against")
	cmd.Flags().StringVar(&f.sql, "sql", "", "the statement to run")
	cmd.Flags().StringVar(&f.file, "file", "", "read the statement from FILE (\"-\" means stdin)")
	return cmd
}

func newCrudCmd(e *env.Env, mode *string) *cobra.Command {
	var f crudFlags
	cmd := &cobra.Command{
		Use:   "crud --db ID --action select|insert|update|delete --resource TABLE",
		Short: "Select, insert, update or delete rows of a table",
		Long: "The action and resource name the call; --filter and --data are JSON sent\n" +
			"inside the payload of the query service.",
		Args: exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := f.validate(); err != nil {
				return err
			}
			m, err := parseMode(*mode)
			if err != nil {
				return err
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			return runCrud(cmd.Context(), e, cl, f, m)
		},
	}
	cmd.Flags().StringVar(&f.db, "db", "", "database id")
	cmd.Flags().StringVar(&f.action, "action", "", "one of select, insert, update or delete")
	cmd.Flags().StringVar(&f.resource, "resource", "", "table name")
	cmd.Flags().StringVar(&f.filter, "filter", "", "JSON object of filter conditions")
	cmd.Flags().StringVar(&f.data, "data", "", "JSON body of the row(s) to write")
	cmd.Flags().StringVar(&f.dataFile, "data-file", "", "read that JSON from FILE (\"-\" means stdin)")
	cmd.Flags().BoolVar(&f.dataStdin, "data-stdin", false, "read that JSON from stdin")
	return cmd
}

func newEnginesCmd(e *env.Env, mode *string) *cobra.Command {
	return &cobra.Command{
		Use:   "engines",
		Short: "List the database engines the query service supports",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			m, err := parseMode(*mode)
			if err != nil {
				return err
			}
			var out any
			req := httpx.Request{Method: "GET", Path: enginesPath, Mode: m}
			if err := cl.JSON(cmd.Context(), req, &out); err != nil {
				return httpx.NotEnabled("query engines", err)
			}
			return printResult(e, out)
		},
	}
}

func newSchemaCmd(e *env.Env, mode *string) *cobra.Command {
	var db string
	cmd := &cobra.Command{
		Use:   "schema --db ID",
		Short: "Show the tables and columns of a database",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if db == "" {
				return exitcode.Usagef("--db is required")
			}
			cl, _, err := e.Client()
			if err != nil {
				return err
			}
			m, err := parseMode(*mode)
			if err != nil {
				return err
			}
			var out any
			path := "/query/v1/" + httpx.Seg(db) + "/schema"
			req := httpx.Request{Method: "GET", Path: path, Mode: m}
			// A 404 here is about this database id, so it stays a plain "not found".
			if err := cl.JSON(cmd.Context(), req, &out); err != nil {
				return err
			}
			return printResult(e, out)
		},
	}
	cmd.Flags().StringVar(&db, "db", "", "database id (see `grobase db list`)")
	return cmd
}
