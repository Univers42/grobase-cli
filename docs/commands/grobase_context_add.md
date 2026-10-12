## grobase context add

Add (or replace) a context; pins the server certificate on first use when it is untrusted

```
grobase context add NAME --url URL [flags]
```

### Options

```
      --ca-file string         PEM CA bundle to trust in addition to the system roots
  -h, --help                   help for add
      --insecure-skip-verify   DANGEROUS: do not verify the server certificate at all
      --pin-sha256 string      pin the server leaf certificate (hex SHA-256 of its DER)
      --ssh grobase host       ssh destination of the machine running grobase, for grobase host (alias or user@host)
      --url stringArray        edge URL, e.g. https://host:8443 (repeatable; all URLs must present the same certificate)
      --yes                    accept the presented certificate fingerprint without asking
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

* [grobase context](grobase_context.md)	 - Manage deployments (contexts)

