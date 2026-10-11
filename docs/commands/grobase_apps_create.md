## grobase apps create

Create an app (its API key is printed once)

```
grobase apps create NAME [--engine ENGINE] [flags]
```

### Options

```
      --engine string   database engine for the app (default: the server's)
  -h, --help            help for create
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

* [grobase apps](grobase_apps.md)	 - Manage the account's self-serve apps

