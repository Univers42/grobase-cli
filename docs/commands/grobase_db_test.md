## grobase db test

Prove one mount answers: the query service opens it and lists its tables

### Synopsis

Ask the query service for the mount's tables (GET /query/v1/{id}/tables): it
resolves the mount under the caller's tenant, decrypts its credentials and opens
the engine, so an answer proves the whole data path. A 404 means the mount does
not exist for this tenant.

```
grobase db test <id> [flags]
```

### Options

```
  -h, --help   help for test
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

