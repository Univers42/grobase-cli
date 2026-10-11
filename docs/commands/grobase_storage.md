## grobase storage

Manage the buckets and objects of the current tenant

### Synopsis

An object is addressed as BUCKET/KEY. A key may contain slashes; the remote
side of `storage cp` is written grobase:BUCKET/KEY.

### Options

```
  -h, --help   help for storage
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
* [grobase storage buckets](grobase_storage_buckets.md)	 - Manage the buckets of the current tenant
* [grobase storage cp](grobase_storage_cp.md)	 - Copy an object between the local filesystem and a bucket
* [grobase storage ls](grobase_storage_ls.md)	 - List the objects of a bucket, optionally restricted to a prefix
* [grobase storage rm](grobase_storage_rm.md)	 - Delete objects from a bucket
* [grobase storage sign](grobase_storage_sign.md)	 - Create a time-limited URL that serves one object

