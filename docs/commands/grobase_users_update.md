## grobase users update

Change the email, role, password or metadata of a user

```
grobase users update <id> [flags]
```

### Options

```
      --email string      new email address
  -h, --help              help for update
      --metadata string   new user metadata as a JSON object
      --password-stdin    read the new password from stdin ($GROBASE_USER_PASSWORD also works)
      --role string       new role claim
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

