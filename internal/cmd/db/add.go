package db

import (
	"context"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Univers42/grobase-cli/internal/env"
	"github.com/Univers42/grobase-cli/internal/exitcode"
	"github.com/Univers42/grobase-cli/internal/httpx"
)

// connEnv is the environment variable holding a connection string, as the
// secrets rule requires: never a flag value.
const connEnv = "GROBASE_DB_CONNECTION_STRING"

// mountSpec is the POST /v1/tenants/me/mounts body.
type mountSpec struct {
	Engine           string `json:"engine"`
	Name             string `json:"name"`
	Isolation        string `json:"isolation,omitempty"`
	ConnectionString string `json:"connection_string,omitempty"`
	CredentialRef    string `json:"credential_ref,omitempty"`
}

type addFlags struct {
	name, engine, isolation, credentialRef string
	connectionStringStdin                  bool
}

func newAddCmd(e *env.Env) *cobra.Command {
	var f addFlags
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Register a database mount for the current tenant",
		Args:  exitcode.Args(cobra.NoArgs),
		RunE:  func(cmd *cobra.Command, _ []string) error { return runAdd(cmd.Context(), e, f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.name, "name", "", "mount name (required)")
	fl.StringVar(&f.engine, "engine", "", "engine, e.g. postgresql (required)")
	fl.StringVar(&f.isolation, "isolation", "", "isolation mode, e.g. schema_per_tenant")
	fl.StringVar(&f.credentialRef, "credential-ref", "", "reference to stored credentials instead of an inline connection string")
	fl.BoolVar(&f.connectionStringStdin, "connection-string-stdin", false,
		"read the connection string from stdin (or $"+connEnv+")")
	return cmd
}

func runAdd(ctx context.Context, e *env.Env, f addFlags) error {
	f.name, f.engine = strings.TrimSpace(f.name), strings.TrimSpace(f.engine)
	if f.name == "" || f.engine == "" {
		return exitcode.Usagef("--name and --engine are required")
	}
	if f.credentialRef != "" && f.connectionStringStdin {
		return exitcode.Usagef("--credential-ref and --connection-string-stdin are mutually exclusive")
	}
	conn, err := readConnectionString(e, f)
	if err != nil {
		return err
	}
	cl, _, err := e.Client()
	if err != nil {
		return err
	}
	body := mountSpec{
		Engine: f.engine, Name: f.name, Isolation: strings.TrimSpace(f.isolation),
		ConnectionString: conn, CredentialRef: strings.TrimSpace(f.credentialRef),
	}
	var out map[string]any
	req := httpx.Request{Method: http.MethodPost, Path: mountsPath(), JSON: body}
	if err := cl.JSON(ctx, req, &out); err != nil {
		return httpx.NotEnabled("database mounts", err)
	}
	return printResult(e, withoutSecrets(out))
}

// readConnectionString reads the DSN without ever taking it from argv: stdin,
// $GROBASE_DB_CONNECTION_STRING or a no-echo prompt. --credential-ref replaces
// it, and with no source at all the mount is registered without one.
func readConnectionString(e *env.Env, f addFlags) (string, error) {
	if f.credentialRef != "" || (!f.connectionStringStdin && e.Getenv(connEnv) == "") {
		return "", nil
	}
	s, err := e.ReadSecret("Connection string: ", f.connectionStringStdin, connEnv)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}
