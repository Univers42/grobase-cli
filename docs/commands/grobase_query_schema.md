## grobase query schema

Show the tables and columns of a database

```
grobase query schema --db ID [flags]
```

### Options

```
      --db grobase db list   database id (see grobase db list)
  -h, --help                 help for schema
```

### Options inherited from parent commands

```
      --context string            context to use (default: the current context)
      --credential-store string   credential backend: auto, keyring or file (default "auto")
      --debug                     trace HTTP requests to stderr (credentials are redacted)
      --jq string                 filter the result with a jq expression (prints JSON lines)
      --mode string               credential to send: auto (a stored API key before the session), user, service, apikey, anon or none (default "auto")
  -o, --output string             output format: table, json or yaml (default "table")
      --timeout duration          time to wait for response headers (default 30s)
```

### SEE ALSO

* [grobase query](grobase_query.md)	 - Run SQL and row CRUD against a mounted database

