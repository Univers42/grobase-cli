## grobase host logs

Print a container's recent logs, credentials redacted

### Synopsis

Prints the last --tail lines of one container (a compose service such as waf, or
its container name), its stdout and stderr together. Each line goes through the
same redaction as --debug: a JSON log line has every credential-named field
masked, any line its JWTs, keys and URL passwords. Caveat: a secret in a shape
and under a name neither layer knows still prints; treat the output as sensitive.

```
grobase host logs <service> [flags]
```

### Options

```
  -h, --help           help for logs
      --since string   only lines newer than this duration (30s, 10m, 2h)
      --tail int       number of lines from the end (default 200)
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

