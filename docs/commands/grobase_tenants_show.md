## grobase tenants show

Show the current tenant

```
grobase tenants show [flags]
```

### Options

```
  -h, --help   help for show
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

* [grobase tenants](grobase_tenants.md)	 - Show the current tenant's profile, resource usage and entitlements

