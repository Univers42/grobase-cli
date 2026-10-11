## grobase completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(grobase completion zsh)

To load completions for every new session, execute once:

#### Linux:

	grobase completion zsh > "${fpath[1]}/_grobase"

#### macOS:

	grobase completion zsh > $(brew --prefix)/share/zsh/site-functions/_grobase

You will need to start a new shell for this setup to take effect.


```
grobase completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
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

* [grobase completion](grobase_completion.md)	 - Generate the autocompletion script for the specified shell

