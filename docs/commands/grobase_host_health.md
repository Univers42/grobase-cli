## grobase host health

Exit 1 naming each container that is unhealthy, crash-looping or exited non-zero

```
grobase host health [flags]
```

### Options

```
  -h, --help   help for health
```

### Options inherited from parent commands

```
      --context string            context to use (default: the current context)
      --credential-store string   credential backend: auto, keyring or file (default "auto")
      --debug                     trace HTTP requests to stderr (credentials are redacted)
      --jq string                 filter the result with a jq expression (prints JSON lines)
  -o, --output string             output format: table, json or yaml (default "table")
      --project string            docker compose project of the grobase stack (default "mini-baas")
      --ssh string                ssh destination (default: the context's ssh)
      --timeout duration          time to wait for response headers (default 30s)
```

### SEE ALSO

* [grobase host](grobase_host.md)	 - Inspect and restart the containers of the machine running grobase (over ssh)

