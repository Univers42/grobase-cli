## grobase keys

Manage the current tenant's API keys

### Synopsis

API keys authenticate machine clients. A key's cleartext value is shown
only once, when it is created or rotated; store it immediately.

### Options

```
  -h, --help   help for keys
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
* [grobase keys create](grobase_keys_create.md)	 - Create an API key (its cleartext is printed once)
* [grobase keys list](grobase_keys_list.md)	 - List the current tenant's API keys
* [grobase keys revoke](grobase_keys_revoke.md)	 - Revoke an API key
* [grobase keys rotate](grobase_keys_rotate.md)	 - Create a replacement key, print it, then revoke the old one

