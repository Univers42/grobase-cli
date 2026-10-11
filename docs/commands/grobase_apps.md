## grobase apps

Manage the account's self-serve apps

### Synopsis

Each app is a new tenant with its own database and a scoped API key, shown once when it
is created; store it immediately. Apps need `grobase login` and a server with self-serve
apps enabled.

### Options

```
  -h, --help   help for apps
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
* [grobase apps create](grobase_apps_create.md)	 - Create an app (its API key is printed once)
* [grobase apps delete](grobase_apps_delete.md)	 - Delete an app and the database it owns
* [grobase apps list](grobase_apps_list.md)	 - List the account's apps

