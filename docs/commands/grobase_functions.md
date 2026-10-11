## grobase functions

List, deploy, invoke and read the logs of edge functions

### Options

```
  -h, --help   help for functions
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
* [grobase functions deploy](grobase_functions_deploy.md)	 - Upload the source of a function (streamed from FILE)
* [grobase functions invoke](grobase_functions_invoke.md)	 - Call a function and print its response
* [grobase functions list](grobase_functions_list.md)	 - List the function triggers of the deployment (needs the service role)
* [grobase functions logs](grobase_functions_logs.md)	 - Show the recent log lines of a function

