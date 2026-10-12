## grobase users sessions revoke

Sign a user out of every device

```
grobase users sessions revoke <id|email> [flags]
```

### Options

```
  -h, --help   help for revoke
      --yes    revoke without asking first
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

* [grobase users sessions](grobase_users_sessions.md)	 - Handle the sessions of a user

