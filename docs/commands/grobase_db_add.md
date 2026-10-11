## grobase db add

Register a database mount for the current tenant

```
grobase db add [flags]
```

### Options

```
      --connection-string-stdin   read the connection string from stdin (or $GROBASE_DB_CONNECTION_STRING)
      --credential-ref string     reference to stored credentials instead of an inline connection string
      --engine string             engine, e.g. postgresql (required)
  -h, --help                      help for add
      --isolation string          isolation mode, e.g. schema_per_tenant
      --name string               mount name (required)
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

