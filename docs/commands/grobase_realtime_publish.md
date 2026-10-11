## grobase realtime publish

Broadcast one message on a channel

### Synopsis

The payload is data, not a secret, so --data may carry it directly;
--data-stdin reads it from standard input instead.

```
grobase realtime publish CHANNEL --event EVENT (--data JSON | --data-stdin) [flags]
```

### Options

```
      --data string    message payload as JSON
      --data-stdin     read the JSON payload from stdin
      --event string   event type of the message (required)
  -h, --help           help for publish
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

* [grobase realtime](grobase_realtime.md)	 - Subscribe to and publish on realtime channels over WebSocket

