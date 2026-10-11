## grobase functions deploy

Upload the source of a function (streamed from FILE)

```
grobase functions deploy NAME --file FILE [flags]
```

### Options

```
      --content-type string   Content-Type of the upload (default "application/javascript")
      --file string           source file to upload (required)
  -h, --help                  help for deploy
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

