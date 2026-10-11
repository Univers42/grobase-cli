## grobase context pin

Re-pin the server certificate of a context (after a rotation)

```
grobase context pin [NAME] [flags]
```

### Options

```
  -h, --help   help for pin
      --yes    accept the presented fingerprint without asking
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

* [grobase context](grobase_context.md)	 - Manage deployments (contexts)

