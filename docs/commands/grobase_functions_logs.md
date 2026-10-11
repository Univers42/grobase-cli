## grobase functions logs

Show the recent log lines of a function

```
grobase functions logs NAME [--tail N] [flags]
```

### Options

```
  -h, --help       help for logs
      --tail int   number of lines (default 100)
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

* [grobase functions](grobase_functions.md)	 - List, deploy, invoke and read the logs of edge functions

