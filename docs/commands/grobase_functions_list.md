## grobase functions list

List the function triggers of one tenant (needs the service role)

```
grobase functions list --tenant ID [flags]
```

### Options

```
  -h, --help            help for list
      --tenant string   tenant whose function triggers to list (required)
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

