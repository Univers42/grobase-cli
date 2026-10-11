## grobase api

Call any endpoint of the current context

### Synopsis

METHOD is sent upper-cased and PATH must start with "/".

Fields come from -f (always a string) and -F (true/false/null and numbers are
typed, anything else stays a string). A GET carries them in the query string,
every other method sends them as a JSON body. --input FILE sends a raw body
("-" is stdin) and wins over fields.

--paginate walks the results: the Link header with rel="next" is followed, and a
PostgREST-style Content-Range answer ("0-24/100") is continued by re-sending the
request with the next offset. Array answers are merged into one result.
--include prints the status line and headers to stderr, --raw writes the body
exactly as received.

```
grobase api METHOD PATH [flags]
```

### Options

```
  -F, --field stringArray       typed field, key=value: true/false/null/number, else string (repeatable)
  -H, --header stringArray      extra header, 'Name: value' (repeatable)
  -h, --help                    help for api
      --include                 print the status line and headers to stderr
      --input string            raw body from FILE ("-" means stdin)
      --max-pages int           stop after that many pages (with --paginate) (default 100)
      --mode string             credential to send: auto, user, service, apikey, anon or none (default "auto")
      --paginate                follow every page until the results are exhausted
      --raw                     print the response body exactly as received
  -f, --raw-field stringArray   string field, key=value (repeatable)
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

