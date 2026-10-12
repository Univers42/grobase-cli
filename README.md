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
grobase signup --email you@example.com --password-stdin       # creates the account (and signs in when the server auto-confirms)
grobase login                                                 # prompts; or: echo "$PW" | grobase login --email me@x.io --password-stdin
grobase whoami
grobase users list --jq '.[].email'
grobase api /v1/tenants/me
```

Secrets are never flag values: they come from a prompt, stdin or an environment variable.

Connecting from another device (LAN, wifi, Tailscale, Funnel, ssh tunnel) with
one context and a pinned certificate: [docs/CONNECTING.md](docs/CONNECTING.md).
`scripts/check-routes.sh` tests every route from the device it runs on. Behind a
proxy (`HTTPS_PROXY`, including Tailscale's userspace SOCKS5), the CLI uses it
for requests and for the certificate check.

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

`api`, `apps`, `completion`, `context`, `db`, `doctor`, `functions`, `host`, `keys`, `login`, `logout`, `orgs`, `query`, `realtime`, `signup`, `status`, `storage`, `tenants`, `token`, `users`, `version`, `whoami`.

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

## Migrations, the host, and 404s

- `db migrate --tenant T --mount NAME DIR|FILE...` applies `.sql` files through `/admin/v1/migrate` with the service-role key. Each file is one migration named by its file stem; a directory gives its `*.sql` in name order. The server records names in `_baas_migrations`, so a re-run reports `skipped`. The data plane resolves the mount's DSN itself from the adapter-registry, so no connection string is read or sent. `db list --tenant T` lists that tenant's mounts the same way.
- `host ps|health|logs|restart` runs fixed `docker` command lines on the grobase machine over the system `ssh` (`--ssh` or the context's `ssh`). `health` exits 1 naming each container that is unhealthy, crash-looping or exited non-zero; `logs` output is redacted line by line; `restart` asks unless `--yes`.
- A 404 says who answered it: `not deployed` (Kong has no route for the path), `switched off` (the service behind the route has no handler for it), or `not enabled` (a bare 404 that names neither). `status`, `doctor` and every feature error use the same verdict.

## What it does not do

- `db migrate` needs a grobase whose data plane has `DATA_PLANE_ADAPTER_REGISTRY_TOKEN` (grobase 8fa693d2 sets it from `ADAPTER_REGISTRY_SERVICE_TOKEN`; before, every migration answered 502 `credential_unavailable`) and whose WAF carries rule 10120 (9f26e792; before, SQL that inserts data answered 403). Only engines with the `ddl` capability migrate (postgresql, mysql); others answer 422. There is no `down`: write a new forward migration. Kong limits a migration body to 256 KB.
- `host` does not recreate, rebuild, pull or `compose up/down`, and it does not change an engine's configuration: `docker restart` keeps the container as it was. The ssh login must be allowed to run docker.
- "switched off" cannot be told from "this CLI and the server disagree on the path": grobase answers both with the same bytes on purpose. A JSON 404 is not classified (GoTrue and PostgREST answer an unknown path and a missing row alike) and keeps the "not enabled" wording.
- `functions deploy` has no `--runtime` flag. `functions logs` cannot tell a missing function from a missing route (both 404).
- A certificate pin skips expiry and hostname checks by design (the pin is the trust anchor); `doctor` reports expiry separately.
- Redaction masks every secret the CLI loaded, every value under a credential-named JSON key at any depth, and JWT, `mbk_`, URL-password, Bearer and PEM shapes. A secret the CLI never held, under an innocuous key or in non-JSON text in no known shape, still passes.
- The file-store passphrase is not confirmed on first creation.
- Cobra's "unknown command/flag" errors are mapped to exit 2 by message prefix.

## Development

```sh
gofumpt -l . && go vet ./... && golangci-lint run && go test -race ./...
```
