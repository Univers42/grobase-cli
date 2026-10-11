## grobase context

Manage deployments (contexts)

### Options

```
  -h, --help   help for context
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
* [grobase context add](grobase_context_add.md)	 - Add (or replace) a context; pins the server certificate on first use when it is untrusted
* [grobase context env](grobase_context_env.md)	 - Print shell assignments for the chosen URL of a context
* [grobase context list](grobase_context_list.md)	 - List contexts
* [grobase context pin](grobase_context_pin.md)	 - Re-pin the server certificate of a context (after a rotation)
* [grobase context remove](grobase_context_remove.md)	 - Remove a context and its stored credentials
* [grobase context show](grobase_context_show.md)	 - Show one context (default: current)
* [grobase context use](grobase_context_use.md)	 - Select the current context

