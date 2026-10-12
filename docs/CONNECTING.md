# Connecting to the grobase data center

How a second machine reaches the grobase deployment on the born2root VM
(`b2b`), with the `grobase` CLI and with groot's `make link`. Every route
below says what was run, when, and what it returned. Routes marked
**not verified** have not been run end to end; do not rely on them without
testing them first.

Verified as of 2026-10-12 unless a row says otherwise.

## 1. What you are connecting to

| Item | Value |
|---|---|
| Host (runs the VM) | LAN `192.168.1.132` (`enp7s0`, DHCP), Tailscale `100.94.119.97` |
| VM | `b2b`, QEMU, grobase at `/opt/grobase` |
| Tailnet | `tail10f2ea.ts.net`, node `dlesieur42-1` |
| WAF (what the CLI talks to) | `https://<address>:8443` |
| ssh into the VM | port `4242`, login `dlesieur` (key only for other devices) |
| VM CA | `/opt/grobase/certs/track-binocle-local-ca.pem`, copy `~/grobase-ca.pem` |
| Leaf certificate pin | `28e27b66a18a90d701a406fbb479c071a26d08cc2fe250a7bb329d327e780c7a` (expires 2027-11-12) |

The WAF listens on the host's loopback, Tailscale and LAN addresses only.
Nothing is published to the public internet except the Funnel route in §2.5.

## 2. The routes

| # | Route | URL | Works from | Status |
|---|---|---|---|---|
| 2.1 | Same host | `https://127.0.0.1:8445` (after the 2026-10-12 reboot; `8443` before) | the host | verified (doctor, 2 ms) |
| 2.2 | LAN IP, pinned | `https://192.168.1.132:8443` | same wifi/ethernet, no Tailscale | verified (`doctor` 10/10, 43 ms, 2026-10-12) |
| 2.3 | LAN name, hosts file + CA | `https://grobase.track-binocle.test:8443` | same wifi, no Tailscale | verified with `curl` (2026-10-12) and CLI signup (2026-10-11); **not verified from a second device** |
| 2.4 | Tailscale IP | `https://100.94.119.97:8443` | any network with Tailscale | verified (CLI, 2026-10-11; `curl` 401 = WAF answers, 2026-10-12) |
| 2.5 | Tailscale name (MagicDNS) | `https://dlesieur42-1.tail10f2ea.ts.net:8443` | a device whose DNS resolves ts.net | verified through `curl --resolve` (2026-10-12); **not verified through DNS from this host** (see §7) |
| 2.6 | Funnel (public) | `https://dlesieur42-1.tail10f2ea.ts.net` (443) | any internet connection | verified with `curl --resolve` to the public address 176.58.90.46, returns 401 from the WAF (2026-10-12); **CLI through Funnel not verified**; **not verified from outside the owner's network** |
| 2.7 | ssh tunnel | `https://127.0.0.1:18443` via `ssh -L` | a network that allows only ssh out | verified through `doctor` (2026-10-12, 4 ms) |
| 2.8 | ssh for `make link` | `ssh://dlesieur@100.94.119.97:4242` | any device on the tailnet with a key on the VM | verified (2026-10-11 runbook); **a second device not verified** |

Failover: a context can list several URLs. The CLI probes them in order and
uses the first that answers `GET /auth/v1/health`; a URL that does not answer
costs `GROBASE_PROBE_TIMEOUT` (default 2 s). Verified 2026-10-12: a dead
first URL fell through to an alternate, `doctor` named the alternate in its
`via` column.

Not verified at all: WiFi from a network other than the owner's, and any
device other than this host. The steps for those are in §5 and §6; they are
the standard steps, not a tested recipe.

## 3. Prerequisites

1. **The CLI.** Build from source, using Go 1.26.9 (the `go.mod` minimum):

   ```sh
   git clone https://github.com/Univers42/grobase-cli
   cd grobase-cli
   docker run --rm -v "$PWD":/src -w /src golang:1.26.9 \
       go build -buildvcs=false -o /src/grobase ./cmd/grobase
   install -m 0755 grobase ~/.local/bin/grobase
   grobase version
   ```

   `-buildvcs=false` is needed when building inside a container; without it
   the build fails with `error obtaining VCS status`.

2. **The VM CA.** Copy it from the VM, then check the fingerprint against the
   VM's own copy. The certificate is public; its key never leaves the VM.

   ```sh
   scp -P 4242 dlesieur@100.94.119.97:/opt/grobase/certs/track-binocle-local-ca.pem ~/grobase-ca.pem
   openssl x509 -noout -fingerprint -sha256 -in ~/grobase-ca.pem
   ssh -p 4242 dlesieur@100.94.119.97 \
       'openssl x509 -noout -fingerprint -sha256 -in /opt/grobase/certs/track-binocle-local-ca.pem'
   ```

   The two lines must match. Last recorded value: `7B:22:BB:3A:…:2F:EE`
   (full value in the runbook). Never use `--insecure-skip-verify` or `curl -k`.

3. **ssh key (only for `make link` over ssh, and for the VM).** From the new
   device, once, over Tailscale:

   ```sh
   ssh-keygen -t ed25519            # if you have no key yet
   ssh-copy-id -p 4242 dlesieur@100.94.119.97
   ssh -p 4242 dlesieur@100.94.119.97 true && echo key-ok
   ```

   The VM password is for the owner only; use the key.

4. **The API keys.** The anon key is needed by GoTrue for every login. It is
   stored on the VM in `/opt/grobase/.env` as `ANON_KEY` (set, verified by
   name only). Read it straight into the CLI so the value never lands in your
   shell history or on screen:

   ```sh
   ssh -p 4242 dlesieur@100.94.119.97 'sed -n "s/^ANON_KEY=//p" /opt/grobase/.env'
   ```

   The service-role key (`SERVICE_ROLE_KEY`, also set) is needed only for
   admin commands such as `grobase users list`. Read it with
   `--service-role` on stdin; never store it in a shell variable or a file.

## 4. The first connection with the CLI (this is the one that matters)

```sh
# 1. one context, every route in the order to try them
grobase context add b2b \
  --url https://192.168.1.132:8443 \
  --url https://dlesieur42-1.tail10f2ea.ts.net:8443 \
  --url https://100.94.119.97:8443 \
  --pin-sha256 28e27b66a18a90d701a406fbb479c071a26d08cc2fe250a7bb329d327e780c7a \
  --yes

# 2. first time only: create the account (the VM auto-confirms emails, so
#    signup signs you in at once). The anon key is read first, then the password.
read -rs -p "password: " PASS; echo
{ ssh -p 4242 dlesieur@100.94.119.97 'sed -n "s/^ANON_KEY=//p" /opt/grobase/.env'
  printf '%s\n' "$PASS"; } |
  grobase signup --email you@example.com --anon-key-stdin --password-stdin
unset PASS

# 3. every later time
grobase login --email you@example.com --password-stdin --anon-key-stdin

# 4. check
grobase whoami
grobase doctor            # the reachability row names the URL in use
grobase status            # every endpoint of the context
```

Notes:

- Use `--ca-file ~/grobase-ca.pem` instead of `--pin-sha256` if you want the
  name in the certificate to be checked. Then every URL must be a name the
  certificate covers (`grobase.track-binocle.test`, `*.track-binocle.test`,
  the ts.net name, `100.94.119.97`, `127.0.0.1`); the bare LAN IP is not in it,
  so a LAN IP URL needs the pin. Verified 2026-10-12: a context with
  `--ca-file` and the LAN IP fails TLS; with `--pin-sha256` it passes.
- `grobase context show b2b` displays the first stored URL, not the one in
  use. Use `grobase context env b2b` to see the URL that was chosen, or
  `grobase doctor`.
- `--pin-sha256` takes the bare 64 hex digits. The `sha256:` prefix is
  rejected.
- The credential store is mode 600. `doctor`'s `tls` row PASSes with the pin
  and only WARNs when the CA is not a system root.
- When the leaf certificate is re-issued (it expires 2027-11-12) `doctor`
  FAILs the `tls` row and shows both fingerprints. Check the new value on the
  VM with `openssl x509 -noout -fingerprint -sha256 -in /opt/grobase/certs/localhost.pem`,
  then run `grobase context pin b2b`.

Other useful commands, all verified against the live deployment:

```sh
grobase api GET /auth/v1/health            # any endpoint; METHOD is required
grobase api GET /query/v1/engines
grobase api POST /path --input body.json    # labels the body application/json
grobase realtime subscribe pg/<table>/updated
```

## 5. From another device: the paths in order

Try them in this order. Each step tells you what to look for.

**a. Same wifi as the host, no Tailscale.** Use route 2.2 (LAN IP with the
pin). `grobase doctor` should show `via https://192.168.1.132:8443`. If the
LAN edge is down, the row shows a timeout and the next URL in the context is
tried. Not verified from a second device.

**b. Same wifi, LAN name.** Add a hosts entry on the device:

- Linux/macOS: `sudo sh -c 'echo "192.168.1.132 grobase.track-binocle.test" >> /etc/hosts'`
- Windows: add the same line to `C:\Windows\System32\drivers\etc\hosts` as administrator.

Then use `~/grobase-ca.pem` as `--ca-file` with the name URL. Verified from
this host with `curl --resolve` (2026-10-12); not verified from a second
device.

**c. Any network, Tailscale.** Install Tailscale on the device and sign in
with the same account that owns the tailnet `tail10f2ea.ts.net` (the owner
approves the device in the admin console, or shares the node). Then route
2.4 (IP) works immediately; route 2.5 (name) works if the device's DNS uses
Tailscale's resolver (MagicDNS is on). **Not verified from a second device.**

**d. Outside, public internet.** Route 2.6 (Funnel) answers with a public
certificate, so the CLI needs no CA file. `curl https://dlesieur42-1.tail10f2ea.ts.net/auth/v1/health`
returns 401 from the WAF (the health route needs the anon key; a 401 proves
the path). **The CLI through Funnel has not been run.** Funnel publishes the
WAF to the internet, which is a decision for the owner.

**e. Only ssh allowed out.** Open the tunnel and point a context at it:

```sh
ssh -N -L 127.0.0.1:18443:127.0.0.1:8443 -p 4242 dlesieur@100.94.119.97
grobase context add tunnel --url https://127.0.0.1:18443 --pin-sha256 28e27b66a18a90d701a406fbb479c071a26d08cc2fe250a7bb329d327e780c7a --yes
```

Verified 2026-10-12: the tunnel URL answered `doctor` (4 ms) as one URL of the
`b2b` context. The separate `tunnel` context above is the same command pattern
but has not been run as its own context.

## 6. groot: the `make link` steps

groot runs its frontends against grobase through a link. Clone it fresh:

```sh
git clone --recursive -b feat/grobase-datacenter-link <groot-url> groot
cd groot
```

Do **not** run plain `make all` on this branch: it fast-forwards the
submodules and drops the pinned grobase and osionos commits the link needs.
Use `make all SKIP_SYNC=1`, or skip `make all` and run the link targets.

Create `.env.grobase-link` (git-ignored) with the route you chose:

```sh
GROBASE_TARGET=ssh://dlesieur@100.94.119.97:4242     # route 2.8, the default that was verified
# or GROBASE_TARGET=https://100.94.119.97:8443       # direct, over Tailscale
GROBASE_CA_FILE=~/grobase-ca.pem
```

Then:

```sh
make link                 # writes the link, starts the relay
make link-frontends-up
make link-verify          # "verify: N frontends reach grobase through the … link"
make link-status          # exit 0 when healthy
```

Verified 2026-10-11: `make link` (9 frontends, direct), `make link-e2e` (11
passed). Verified 2026-10-12: the self-heal drill restarted the link and
`link-verify` passed (6 frontends, ssh). The `ssh://` target is the one
verified after the reboot. Direct mode with `GROBASE_CA_FILE` is verified on
the host only.

## 7. Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `doctor` `dns/reachability` FAIL on the ts.net name, `getent hosts` empty | This host's system resolver does not use MagicDNS (100.100.100.100 times out). Verified 2026-10-12. | Use the IP URL on this host. On a device with Tailscale DNS on, the name resolves. |
| TLS verification fails on the LAN IP with `--ca-file` (`curl` returns 000) | `--ca-file` with a LAN IP. The cert has no LAN SAN. Verified 2026-10-12. | Use `--pin-sha256` for that URL, or the hosts name. |
| `tls` FAIL with two fingerprints | Leaf re-issued. | Verify the new fingerprint on the VM, then `grobase context pin b2b`. |
| Dead port 8443 on `127.0.0.1` | After the 2026-10-12 host reboot the VM's forward was on 8445 until the born2root port fix takes effect at the VM's next start. | Use the LAN edge or Tailscale URL, or 8445 on the host. |
| `context show` shows a URL you did not expect | It prints the first stored URL. | `grobase context env b2b` or `grobase doctor`. |
| `grobase users list` 401/403 | Needs the service-role key via `--service-role` on stdin. | Not verified with the service key in this session. |
| Build error `error obtaining VCS status` | Build inside a container without `-buildvcs=false`. | Add the flag. |
| Build error `go.mod requires go >= 1.26.9` | Older Go in the container. | Use `golang:1.26.9`. |

## 8. Verification record

| Check | Route | Date | Result |
|---|---|---|---|
| `grobase doctor` 10/10 | LAN IP pin | 2026-10-12 | pass |
| `grobase doctor`, one dead URL then alternate | failover | 2026-10-12 | pass, 2 s |
| `grobase signup` then `login`, `whoami` | LAN name (signup), Tailscale name (login) | 2026-10-11 | pass |
| `grobase` context over the Tailscale IP, `doctor` | Tailscale IP, CA file | 2026-10-11 | pass |
| `curl --resolve` to the ts.net name | Tailscale name | 2026-10-12 | 401 from WAF |
| `curl --resolve` to the public address | Funnel | 2026-10-12 | 401 from WAF |
| `make link-verify`, `make link-e2e` | groot, direct and ssh | 2026-10-11 | pass |
| `grobase doctor` through `ssh -L` tunnel | tunnel | 2026-10-12 | pass, 4 ms |
| CLI through Funnel | Funnel | — | **not run** |
| CLI or groot from a second physical device | all | — | **not run** |
| Wifi on a network other than the owner's | all | — | **not run** |
| `grobase users list` with the service key | admin | — | **not run** |

Per-run output is in `groot/wiki/runbooks/grobase-link.md` under "Verified".
