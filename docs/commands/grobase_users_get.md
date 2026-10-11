## grobase users get

Show one user by id or by email

```
grobase users get <id|email> [flags]
```

### Options

```
  -h, --help   help for get
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

* [grobase users](grobase_users.md)	 - Manage the users of a deployment (admin: needs the service-role key)

