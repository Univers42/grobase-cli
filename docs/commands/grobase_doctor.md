## grobase doctor

Check the current context: config, connectivity, TLS, clock, credentials, auth and features

### Synopsis

Run the checks in order and report each one as PASS, WARN, FAIL or SKIP.
Exit code is 1 as soon as one check fails. Nothing is written: doctor only
reads the configuration, the credential store and the deployment.

```
grobase doctor [flags]
```

### Options

```
  -h, --help   help for doctor
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

