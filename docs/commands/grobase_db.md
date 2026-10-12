## grobase db

Manage the database mounts of the current tenant

### Options

```
  -h, --help   help for db
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
* [grobase db add](grobase_db_add.md)	 - Register a database mount for the current tenant
* [grobase db list](grobase_db_list.md)	 - List the database mounts of the current tenant, or of --tenant with the service-role key
* [grobase db migrate](grobase_db_migrate.md)	 - Apply SQL migrations to one tenant's database mount (service-role key)
* [grobase db remove](grobase_db_remove.md)	 - Delete a database mount of the current tenant
* [grobase db test](grobase_db_test.md)	 - Prove one mount answers: the query service opens it and lists its tables

