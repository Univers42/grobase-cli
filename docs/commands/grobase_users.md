## grobase users

Manage the users of a deployment (admin: needs the service-role key)

### Synopsis

List, inspect, create, update, ban and delete users, send invitations and revoke sessions.

These calls hit the GoTrue admin API and therefore authenticate with the stored service-role
key (`grobase login --service-role`).

### Options

```
  -h, --help   help for users
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
* [grobase users ban](grobase_users_ban.md)	 - Ban a user
* [grobase users create](grobase_users_create.md)	 - Create a user
* [grobase users delete](grobase_users_delete.md)	 - Delete a user permanently
* [grobase users get](grobase_users_get.md)	 - Show one user by id or by email
* [grobase users invite](grobase_users_invite.md)	 - Send an invitation email
* [grobase users list](grobase_users_list.md)	 - List users
* [grobase users sessions](grobase_users_sessions.md)	 - Handle the sessions of a user
* [grobase users unban](grobase_users_unban.md)	 - Lift the ban of a user
* [grobase users update](grobase_users_update.md)	 - Change the email, role, password or metadata of a user

