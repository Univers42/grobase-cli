## grobase storage ls

List the objects of a bucket, optionally restricted to a prefix

```
grobase storage ls BUCKET [PREFIX] [flags]
```

### Options

```
  -h, --help         help for ls
      --limit int    page size (default 100)
      --offset int   first object of the page
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

* [grobase storage](grobase_storage.md)	 - Manage the buckets and objects of the current tenant

