## grobase db list

List the database mounts of the current tenant, or of --tenant with the service-role key

### Synopsis

Without --tenant, the mounts of the tenant the credential belongs to (tenant
self-service, /v1/tenants/me/mounts). With --tenant, any tenant's mounts through the
service-role admin route, which also works where self-service is switched off.

```
grobase db list [flags]
```

### Options

```
  -h, --help                                  help for list
      --tenant grobase login --service-role   tenant whose mounts to list (needs grobase login --service-role)
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

