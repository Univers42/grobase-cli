## grobase host

Inspect and restart the containers of the machine running grobase (over ssh)

### Synopsis

Runs docker on the machine that hosts grobase, through the system ssh client:
the destination is --ssh, else the context's ssh (grobase context add --ssh).
The ssh login must be allowed to run docker (the docker group). Only fixed
command lines are sent, every name checked first.

### Options

```
  -h, --help             help for host
      --project string   docker compose project of the grobase stack (default "mini-baas")
      --ssh string       ssh destination (default: the context's ssh)
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
* [grobase host health](grobase_host_health.md)	 - Exit 1 naming each container that is unhealthy, crash-looping or exited non-zero
* [grobase host logs](grobase_host_logs.md)	 - Print a container's recent logs, credentials redacted
* [grobase host ps](grobase_host_ps.md)	 - List the stack's containers with their state and health
* [grobase host restart](grobase_host_restart.md)	 - Restart containers of the stack (asks first unless --yes)

