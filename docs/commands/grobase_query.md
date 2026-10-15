## grobase query

Run SQL and row CRUD against a mounted database

### Synopsis

Talk to the query service of the current context:
  run      run one read-only SQL statement (inline, from a file or from stdin)
  crud     select/insert/update/delete rows of a table
  engines  list the database engines the service supports
  schema   show the tables and columns of a database

### Options

```
  -h, --help          help for query
      --mode string   credential to send: auto (a stored API key before the session), user, service, apikey, anon or none (default "auto")
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

* [grobase](grobase.md)	 - Remote controller for a self-hosted grobase platform
* [grobase query crud](grobase_query_crud.md)	 - Select, insert, update or delete rows of a table
* [grobase query engines](grobase_query_engines.md)	 - List the database engines the query service supports
* [grobase query run](grobase_query_run.md)	 - Run one read-only SQL statement against a database
* [grobase query schema](grobase_query_schema.md)	 - Show the tables and columns of a database

