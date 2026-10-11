## grobase orgs invites create

Invite someone to an organisation

```
grobase orgs invites create <orgId> [flags]
```

### Options

```
      --email string   email address to invite (required)
  -h, --help           help for create
      --role string    role to grant, e.g. developer
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

* [grobase orgs invites](grobase_orgs_invites.md)	 - Manage the invitations of an organisation

