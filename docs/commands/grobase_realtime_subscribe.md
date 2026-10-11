## grobase realtime subscribe

Print the messages of one channel as they arrive (NDJSON)

### Synopsis

One message per line on stdout, so the stream pipes into jq or a file.
Without --count the subscription runs until --timeout, the peer closes or the
process is interrupted.

```
grobase realtime subscribe CHANNEL [flags]
```

### Options

```
      --count int          stop after this many messages (0 = no limit)
      --event string       only receive this event type
  -h, --help               help for subscribe
      --timeout duration   stop after this long (0 = no limit)
```

### Options inherited from parent commands

```
      --context string            context to use (default: the current context)
      --credential-store string   credential backend: auto, keyring or file (default "auto")
      --debug                     trace HTTP requests to stderr (credentials are redacted)
      --jq string                 filter the result with a jq expression (prints JSON lines)
  -o, --output string             output format: table, json or yaml (default "table")
```

### SEE ALSO

* [grobase realtime](grobase_realtime.md)	 - Subscribe to and publish on realtime channels over WebSocket

