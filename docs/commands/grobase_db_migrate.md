## grobase db migrate

Apply SQL migrations to one tenant's database mount (service-role key)

### Synopsis

Each .sql file is one migration, named by its file name without .sql; a directory
gives its *.sql files in name order (prefix them 0001_, 0002_, ...). A file is sent
whole, so it may hold several statements: Postgres applies it in one transaction,
MySQL statement by statement (its DDL commits on its own). The server records each
name in the database's _baas_migrations table and skips a name it already holds, so
a re-run is safe and a changed file needs a new name. The run stops at the first
failure, since later files usually build on earlier ones.

Engines that advertise ddl apply migrations (postgresql, mysql); the others are
refused. The server resolves the mount's credentials itself, so no connection
string is read or sent.

```
grobase db migrate --tenant <id> --mount <name|id> <file.sql|dir>... [flags]
```

### Options

```
  -h, --help                             help for migrate
      --mount grobase db list --tenant   mount name or id (required; see grobase db list --tenant)
      --tenant string                    tenant that owns the mount (required)
```

### Options inherited from parent commands

```
      --context string            context to use (default: the current context)
      --credential-store string   credential backend: auto, keyring or file (default "auto")
      --debug                     trace HTTP requests to stderr (credentials are redacted)
      --jq string                 filter the result with a jq expression (prints JSON lines)
  -o, --output string             output format: table, json or yaml (default "table")
      --timeout duration          time to wait for response headers (default 30s)
```

### SEE ALSO

* [grobase db](grobase_db.md)	 - Manage the database mounts of the current tenant

