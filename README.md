# grobase

A command-line remote controller for a [grobase](https://github.com/Univers42) BaaS deployment: contexts, encrypted credentials, users, API keys, database mounts, queries, storage, realtime, functions, orgs, plus `status` and `doctor`. Everything goes through the deployment's gateway (Kong); the CLI never talks to a database or a host directly.

## Install

```sh
go install github.com/Univers42/grobase-cli/cmd/grobase@latest
# or from a checkout
go build -o grobase ./cmd/grobase
```

Requires Go 1.26. Cross-builds (linux/darwin/windows, amd64/arm64) run in CI.

## Quickstart

```sh
grobase context add lab --url https://gateway.example:8443   # shows the cert fingerprint, asks to pin it (TOFU)
grobase status                                                # unauthenticated probe of every endpoint
grobase login                                                 # prompts; or: echo "$PW" | grobase login --email me@x.io --password-stdin
grobase whoami
grobase users list --jq '.[].email'
grobase api /v1/tenants/me
```

Secrets are never flag values: they come from a prompt, stdin or an environment variable.

## Contexts

One deployment is reachable under several URLs depending on the network. A context holds all of them and uses the first that answers:

```sh
grobase context add dc --url https://grobase.track-binocle.test:8443 --url https://100.94.119.97:8443 --url https://127.0.0.1:8443 --ca-file ca.pem
```

`grobase context env` prints `GROBASE_TARGET` and `GROBASE_CA_FILE` for the reachable URL, for `make` and shells to consume.

## Output

Real output, captured against a local fake gateway that only answers `/auth/v1/health`, `/rest/` and `/storage/` (so the rest correctly reads "not enabled"):

```
$ grobase status
ENDPOINT            STATUS  LATENCY_MS  STATE
/auth/v1/health     200     0           ok
/rest/v1/           200     1           ok
/storage/v1/bucket  200     0           ok
/query/v1/engines   404     1           not enabled
/realtime/v1/       404     1           not enabled
/functions/v1/      404     1           not enabled
/v1/tenants/me      404     0           not enabled

$ grobase doctor
CHECK                      RESULT  DETAIL
config                     PASS    context "demo" -> http://127.0.0.1:18080
dns/reachability           PASS    404 Not Found in 0ms
tls                        WARN    plaintext http: traffic can be read and altered on the path
clock skew                 PASS    server clock is -0s away (local clock is ahead), 0ms round trip
credentials                WARN    nothing stored: run `grobase login` or `grobase keys create`
auth                       SKIP    no credential stored: nothing to check
feature /v1/orgs           PASS    not enabled on this deployment
...
doctor: 7 pass, 2 warn, 0 fail, 1 skip
```

`-o table|json|yaml` selects the format; `--jq EXPR` filters the result with gojq.

Exit codes: `0` ok, `1` error, `2` usage, `3` authentication/authorization, `4` not found or not enabled.

## Commands

`api`, `apps`, `completion`, `context`, `db`, `doctor`, `functions`, `keys`, `login`, `logout`, `orgs`, `query`, `realtime`, `status`, `storage`, `tenants`, `token`, `users`, `version`, `whoami`.

The full reference is generated from the cobra tree (`grobase docs DIR`, hidden) and committed in [docs/commands](docs/commands). Regenerate with `grobase docs docs/commands`.

## Security model

See [SECURITY.md](SECURITY.md). In short:

- TLS per context: system roots, a `--ca-file`, a SHA-256 pin of the leaf certificate (set by trust-on-first-use), or `--insecure` (prints a warning on every use; cannot be combined with a CA file or pin).
- Credentials: OS keyring when available, otherwise `credentials.enc` (0600, argon2id-derived key, XChaCha20-Poly1305, header authenticated, fresh nonce per save).
- Sessions refresh automatically, proactively near expiry and once after a 401.
- Redaction applies to all output including `--debug`.

## Dependencies

| Module | Why |
|---|---|
| spf13/cobra | command tree, flags, completion, doc generation |
| zalando/go-keyring | OS keyring access without cgo |
| golang.org/x/crypto | argon2id and XChaCha20-Poly1305 for the file store |
| golang.org/x/term | no-echo password prompts |
| itchyny/gojq | `--jq` without shelling out to jq |
| gopkg.in/yaml.v3 | `-o yaml` |
| coder/websocket | realtime; honours the context's TLS trust |

## What it does not do

- No `migrations` command: grobase's migration route (`/admin/v1/migrate`) is an internal admin route that takes a raw database mount, not a user-facing API. `apps` needs the server's self-serve apps enabled; otherwise it reports not enabled.
- `functions deploy` has no `--runtime` flag. `functions logs` cannot tell a missing function from a missing route (both 404).
- A certificate pin skips expiry and hostname checks by design (the pin is the trust anchor); `doctor` reports expiry separately.
- The "not enabled" wording is inferred from a 404 on a feature root; it cannot distinguish a disabled feature from a wrong path.
- Redaction is pattern-based: a secret in an unusual shape inside a response body can slip through `--debug`.
- The file-store passphrase is not confirmed on first creation.
- Cobra's "unknown command/flag" errors are mapped to exit 2 by message prefix.
- It does not manage the host, containers or the database engines themselves.

## Development

```sh
gofumpt -l . && go vet ./... && golangci-lint run && go test -race ./...
```
