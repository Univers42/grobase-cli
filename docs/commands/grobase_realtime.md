## grobase realtime

Subscribe to and publish on realtime channels over WebSocket

### Synopsis

The socket authenticates with the credentials of the current context, the same
ones the HTTP commands use; nothing is passed on the command line.

### Options

```
  -h, --help   help for realtime
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
* [grobase realtime publish](grobase_realtime_publish.md)	 - Broadcast one message on a channel
* [grobase realtime subscribe](grobase_realtime_subscribe.md)	 - Print the messages of one channel as they arrive (NDJSON)

