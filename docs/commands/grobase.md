## grobase

Remote controller for a self-hosted grobase platform

### Synopsis

grobase manages a grobase deployment from the command line: contexts, credentials, users,
API keys, database mounts, queries, storage, realtime and functions.

Exit codes: 0 ok, 1 error, 2 usage, 3 authentication/authorization, 4 not found or not enabled.

### Options

```
      --context string            context to use (default: the current context)
      --credential-store string   credential backend: auto, keyring or file (default "auto")
      --debug                     trace HTTP requests to stderr (credentials are redacted)
  -h, --help                      help for grobase
      --jq string                 filter the result with a jq expression (prints JSON lines)
  -o, --output string             output format: table, json or yaml (default "table")
      --timeout duration          time to wait for response headers (default 30s)
```

### SEE ALSO

* [grobase api](grobase_api.md)	 - Call any endpoint of the current context
* [grobase completion](grobase_completion.md)	 - Generate the autocompletion script for the specified shell
* [grobase context](grobase_context.md)	 - Manage deployments (contexts)
* [grobase db](grobase_db.md)	 - Manage the database mounts of the current tenant
* [grobase doctor](grobase_doctor.md)	 - Check the current context: config, connectivity, TLS, clock, credentials, auth and features
* [grobase functions](grobase_functions.md)	 - List, deploy, invoke and read the logs of edge functions
* [grobase keys](grobase_keys.md)	 - Manage the current tenant's API keys
* [grobase login](grobase_login.md)	 - Authenticate against the current context and store the credential encrypted
* [grobase logout](grobase_logout.md)	 - Forget the session (and with --all every stored key) of the current context
* [grobase orgs](grobase_orgs.md)	 - Manage organisations, their members, invites and projects
* [grobase query](grobase_query.md)	 - Run SQL and row CRUD against a mounted database
* [grobase realtime](grobase_realtime.md)	 - Subscribe to and publish on realtime channels over WebSocket
* [grobase status](grobase_status.md)	 - Probe every endpoint of the current context and report its state
* [grobase storage](grobase_storage.md)	 - Manage the buckets and objects of the current tenant
* [grobase tenants](grobase_tenants.md)	 - Show the current tenant's profile, resource usage and entitlements
* [grobase token](grobase_token.md)	 - Handle raw credentials (explicit only)
* [grobase users](grobase_users.md)	 - Manage the users of a deployment (admin: needs the service-role key)
* [grobase version](grobase_version.md)	 - Print build information
* [grobase whoami](grobase_whoami.md)	 - Show which credentials are stored and who the server thinks you are

