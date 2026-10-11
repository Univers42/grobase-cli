## grobase orgs members remove

Remove a member from an organisation

```
grobase orgs members remove <orgId> <userId> [flags]
```

### Options

```
  -h, --help   help for remove
      --yes    skip the confirmation prompt
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

* [grobase orgs members](grobase_orgs_members.md)	 - Manage the members of an organisation

