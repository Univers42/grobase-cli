## grobase login

Authenticate against the current context and store the credential encrypted

### Synopsis

Methods (pick one):
  --email E             password from a prompt, --password-stdin or $GROBASE_PASSWORD
  --email E --otp       emailed one-time code / magic link, code from a prompt or stdin
  --service-role        service-role key read from stdin (admin commands)
  --api-key             app API key (mbk_...) read from stdin

The anon key is needed by GoTrue: --anon-key-stdin (read first, before any other stdin
secret) or $GROBASE_ANON_KEY. Secrets are never accepted as flag values.

```
grobase login [flags]
```

### Options

```
      --anon-key-stdin   read the anon (publishable) key from stdin
      --api-key          store an app API key read from stdin
      --email string     account email
  -h, --help             help for login
      --otp              log in with an emailed one-time code instead of a password
      --password-stdin   read the password from stdin
      --service-role     store a service-role key read from stdin
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

