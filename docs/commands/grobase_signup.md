## grobase signup

Create an account and sign in when the deployment auto-confirms emails

### Synopsis

The anon key is needed by GoTrue: --anon-key-stdin (read first, before the password)
or $GROBASE_ANON_KEY. Secrets are never accepted as flag values.

```
grobase signup --email EMAIL [flags]
```

### Options

```
      --anon-key-stdin   read the anon (publishable) key from stdin
      --email string     account email (required)
  -h, --help             help for signup
      --password-stdin   read the password from stdin
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

