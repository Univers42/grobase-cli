## grobase status

Probe every endpoint of the current context and report its state

### Synopsis

GET each endpoint of the current context, concurrently, and report whether it answers.
State is ok (the route answered), down (5xx or no connection), or what a 404 says:
not deployed (the gateway has no route), switched off (the service behind the route
has no handler) or not enabled (a bare 404 that names neither).
Exit code is 1 only when the context URL itself is unreachable; the probes send
no credential, so status works before any `grobase login`.

```
grobase status [flags]
```

### Options

```
  -h, --help   help for status
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

