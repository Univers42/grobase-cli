## grobase keys create

Create an API key (its cleartext is printed once)

```
grobase keys create --name NAME [--scope SCOPE ...] [flags]
```

### Options

```
  -h, --help            help for create
      --name string     human-readable name for the key
      --scope strings   restrict the key to a scope (repeatable)
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

* [grobase keys](grobase_keys.md)	 - Manage the current tenant's API keys

