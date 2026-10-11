## grobase storage buckets delete

Delete a bucket and every object it holds

```
grobase storage buckets delete NAME [flags]
```

### Options

```
  -h, --help   help for delete
      --yes    skip the confirmation prompt
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

* [grobase storage buckets](grobase_storage_buckets.md)	 - Manage the buckets of the current tenant

