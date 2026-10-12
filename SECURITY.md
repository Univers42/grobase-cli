# Security model

`grobase` holds credentials for a BaaS control plane, so this file states what it
protects, what it does not, and where each guarantee is tested.

## Reporting

Open a private security advisory on the GitHub repository. Do not put keys or tokens in issues.

## What is stored where

| Item | Location | Protection |
|---|---|---|
| Contexts (name, URL, CA path, pin) | `config.json` (mode 0600, dir 0700) | no secrets by construction |
| Anon / service-role / API key, GoTrue session | OS keyring (service `grobase-cli`), else `credentials.enc` | keyring ACL, or the file scheme below |

## Credential file (fallback when no keyring is reachable)

- One JSON blob, encrypted with XChaCha20-Poly1305. Key = argon2id(passphrase, 16-byte random salt), default t=3, m=64 MiB, p=4.
- The header (version, KDF parameters, salt) is bound as AAD, so editing any of it fails authentication.
- Fresh 24-byte nonce on every save; written to a temp file then renamed, mode 0600.
- A KDF memory cost above the cap is refused when reading the header, so a hostile file cannot make the CLI allocate gigabytes.
- Passphrase: terminal prompt or `GROBASE_PASSPHRASE`. Wrong passphrase and tampering both surface as an authentication failure (exit 3).
- Tests (`internal/secret/file_test.go`): no plaintext on disk, mode 0600/0700, wrong passphrase, tampered ciphertext/nonce/salt/params, oversized KDF header.

## Secrets never travel in argv

Passwords, keys and connection strings are read from a prompt, `--*-stdin`, or an environment variable. No flag takes a secret value, so none shows up in `ps` or shell history.

## Redaction

Every output path (`--debug` trace, errors, `host logs`) goes through `internal/redact`, in three layers: every secret value the CLI loaded or read, exactly; in a JSON document, every value under a key that names a credential (`token`, `secret`, `password`, `credential`, `authorization`, `cookie`, `signature`, a name ending in `key` or `dsn`, ...) at any depth and whatever its shape, including a JSON document quoted inside a string; then free-text shapes (JWTs, `mbk_` keys, URL passwords, `Bearer` tokens, PEM private keys). Sensitive headers (`Authorization`, `apikey`, cookies) are masked whole. A trace is redacted before it is clipped, so a long JSON body keeps its structure. Tests: `internal/redact`, `internal/httpx` and `internal/cmd/host`. Measured on the baas VM (2026-10-12): the WAF's audit log held the service-role key on 28 lines; `grobase host logs waf` printed it on 0, with 41 masks.
Caveat: a secret the CLI never held, under an innocuous key (`{"value":"..."}`) or in non-JSON text in no recognised shape, is not masked; over-masking (`credential_ref`, `token_type`) is the chosen failure. `token print` deliberately prints a credential; that is its purpose.

## Host access

`grobase host` runs `ssh -o BatchMode=yes -o ConnectTimeout=10 -- DEST 'docker ...'`. The destination must pass `config.ValidSSHTarget` (no leading `-`, so `-oProxyCommand=...` cannot become an ssh option; no spaces or shell syntax), and the remote command line is built from fixed words plus values checked against closed character sets (`--project`, service names, `--since`, `--tail`). A service name is also matched against the stack's own container list before anything but the listing runs. `restart` asks for confirmation unless `--yes`.

## TLS

<a id="tls"></a>
Per context, exactly one trust mode:

1. System roots plus an optional `--ca-file`.
2. `--pin-sha256`: SHA-256 of the server leaf certificate. The pin replaces chain verification and is checked in `VerifyConnection`, so resumed sessions are checked too. Expiry and hostname are not checked: the pinned certificate is the trust anchor (made for self-signed VMs). A re-issued certificate needs `grobase context pin`.
3. `--insecure-skip-verify`: no verification; prints a warning on every `context add`. It cannot be combined with a CA file or pin.

Trust on first use: `context add` against an untrusted server shows subject, issuer, expiry and fingerprint and asks before storing the pin (`--yes` skips the question; only use it when the fingerprint was verified out of band).
Limit: `Probe` reports "verified" against the system roots only, so a server signed by a `--ca-file` CA can show as unverified during TOFU.

## Out of scope

- A compromised account on your own machine (it can read process memory, the keyring and the passphrase env).
- Server-side authorization: the CLI sends your credential; the platform decides what it may do.
- Revoking a leaked service-role key: rotate it on the platform.
