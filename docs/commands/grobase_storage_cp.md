## grobase storage cp

Copy an object between the local filesystem and a bucket

### Synopsis

One side is a local path, the other grobase:BUCKET/KEY. Use - as the local
destination to write the object to standard output.

```
grobase storage cp SRC DST [flags]
```

### Options

```
      --content-type string   Content-Type of an upload (default: guessed from the file name)
  -h, --help                  help for cp
      --overwrite             replace an object that already exists
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

