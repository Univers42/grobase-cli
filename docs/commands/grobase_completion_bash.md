## grobase completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(grobase completion bash)

To load completions for every new session, execute once:

#### Linux:

	grobase completion bash > /etc/bash_completion.d/grobase

#### macOS:

	grobase completion bash > $(brew --prefix)/etc/bash_completion.d/grobase

You will need to start a new shell for this setup to take effect.


```
grobase completion bash
```

### Options

```
  -h, --help              help for bash
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

