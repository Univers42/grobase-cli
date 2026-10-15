## grobase query crud

Select, insert, update or delete rows of a table

### Synopsis

The action and resource name the call; --filter and --data are JSON sent
inside the payload of the query service.

```
grobase query crud --db ID --action select|insert|update|delete --resource TABLE [flags]
```

### Options

```
      --action string      one of select, insert, update or delete
      --data string        JSON body of the row(s) to write
      --data-file string   read that JSON from FILE ("-" means stdin)
      --data-stdin         read that JSON from stdin
      --db string          database id
      --filter string      JSON object of filter conditions
  -h, --help               help for crud
      --resource string    table name
```

### Options inherited from parent commands

```
      --context string            context to use (default: the current context)
      --credential-store string   credential backend: auto, keyring or file (default "auto")
      --debug                     trace HTTP requests to stderr (credentials are redacted)
      --jq string                 filter the result with a jq expression (prints JSON lines)
      --mode string               credential to send: auto (a stored API key before the session), user, service, apikey, anon or none (default "auto")
  -o, --output string             output format: table, json or yaml (default "table")
      --timeout duration          time to wait for response headers (default 30s)
```

### SEE ALSO

* [grobase query](grobase_query.md)	 - Run SQL and row CRUD against a mounted database

