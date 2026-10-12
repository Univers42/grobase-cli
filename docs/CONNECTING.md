# Connecting to the grobase data center from another device

How a second machine reaches the grobase deployment on the born2root VM, with
the `grobase` CLI (and with groot's `make link`). §8 lists every check that was
run and what it returned, and what was not run. Re-run any of them with
`scripts/check-routes.sh` (§6).

## 1. The machines

| Machine | Tailscale node | Addresses | What listens |
|---|---|---|---|
| Host (runs the VM) | `dlesieur42-2`, `100.94.109.44` | LAN `192.168.1.132` | LAN edge `192.168.1.132:8443` → the VM's WAF; the VM's ssh on `192.168.1.132:4242` |
| VM `b2b` (grobase at `/opt/grobase`) | `dlesieur42-1`, `100.94.119.97`, `dlesieur42-1.tail10f2ea.ts.net` | — | WAF `:8443`, ssh `:4242`, Funnel `:443` (public) |

The VM runs Tailscale itself (born2root's `make tailnet`), so the tailnet routes
reach the VM directly, not through the host. Tailnet: `tail10f2ea.ts.net`.

| Item | Value |
|---|---|
| WAF leaf pin (`--pin-sha256`) | `28e27b66a18a90d701a406fbb479c071a26d08cc2fe250a7bb329d327e780c7a`, expires 2027-11-12 |
| VM CA (`--ca-file`) | `/opt/grobase/certs/track-binocle-local-ca.pem` on the VM |
| Account | `cli-firstconnect-130748@example.com` |
| Its password and the anon key | in the host's b2b secrets file as `GROBASE_PASSWORD`, `GROBASE_ANON_KEY` (with `GROBASE_EMAIL`) |

## 2. The routes

| Route | URL / target | Works from | Proven on 2026-10-12 (§8) |
|---|---|---|---|
| `lan-ip` | `https://192.168.1.132:8443` + pin | same LAN / wifi | yes, from a container through the host firewall; not from a physical wifi device |
| `lan-name` | `https://grobase.track-binocle.test:8443` | same LAN, hosts-file entry | only with `curl --resolve` on the host |
| `tailnet-ip` | `https://100.94.119.97:8443` + pin | any network, Tailscale | yes, from a separate tailnet node over the internet relay (DERP) |
| `magicdns` | `https://dlesieur42-1.tail10f2ea.ts.net:8443` + pin | any network, Tailscale with its DNS | yes, same node; not on a machine running hide (§7) |
| `funnel` | `https://dlesieur42-1.tail10f2ea.ts.net` (443, public cert) | any internet connection, no Tailscale | yes, over the public internet |
| `ssh-tailnet` | `ssh -p 4242 dlesieur@100.94.119.97` | any network, Tailscale | yes, key login through the separate node |
| `ssh-lan` | `ssh -p 4242 dlesieur@192.168.1.132` | same LAN | banner, from a container through the host firewall |
| ssh tunnel | `ssh -N -L 127.0.0.1:18443:127.0.0.1:8443 -p 4242 dlesieur@100.94.119.97`, then `https://127.0.0.1:18443` | a network that lets only ssh out | 2026-10-12 earlier, `doctor` through the tunnel |

Funnel publishes the WAF to the whole internet. The pin does not apply to it:
its certificate is a public one, so the CLI trusts it through the system roots.

Funnel carries the user plane only. It reaches the WAF from the VM's loopback,
which Kong's allow list on its private routes admits, so the WAF itself
refuses `/admin/v1`, `/meta/v1` and `/realtime/v1/publish` to every request
Funnel delivers (grobase rule 10130, `1aaac82e`, 2026-10-12). Over `funnel`,
`login`, `whoami`, `doctor`, `api` on the data routes and realtime
subscriptions work. `db migrate`, `functions` and the other service-role admin
commands answer 403 there; run them over the tailnet, the LAN or ssh.

## 3. On the other device: Tailscale

Skip this for the LAN-only and Funnel routes.

**With root (your own laptop):**

```sh
curl -fsSL https://tailscale.com/install.sh | sh     # or the package from tailscale.com/download
sudo tailscale up                                    # opens a login URL
```

Log in with the account that owns the tailnet `tail10f2ea.ts.net` (on the host,
`tailscale whois 100.94.119.97` names it). Then check:

```sh
tailscale status | grep dlesieur42-1                 # the VM node is listed
tailscale ping dlesieur42-1                          # "pong … via DERP(…)" or "via <ip>:<port>"
getent hosts dlesieur42-1.tail10f2ea.ts.net          # 100.94.119.97 = MagicDNS works
```

**Without root (a 42 seat):** run Tailscale in userspace mode. It has no network
interface, so programs reach the tailnet through its proxy. This is the setup
§8 test B ran. Take `tailscale` and `tailscaled` from the static tarball at
`pkgs.tailscale.com/stable/#static` and put both on your `PATH`.

```sh
tailscaled --tun=userspace-networking --socks5-server=127.0.0.1:1055 \
    --state="$HOME/.local/share/tailscale/state" --socket="$HOME/.local/share/tailscale/sock" &
tailscale --socket="$HOME/.local/share/tailscale/sock" up
export HTTPS_PROXY=socks5://127.0.0.1:1055           # the CLI then reaches the tailnet, names included
```

Use a CLI built from commit `7cb3ba9` or later. Before it, `doctor` and
`context add` checked the certificate with a direct connection that ignored
`HTTPS_PROXY`, so they failed in this mode even though requests worked.

## 4. On the other device: the CLI and the account

1. **Install the CLI** (Go 1.26.9 or later; or build it in a container):

   ```sh
   go install github.com/Univers42/grobase-cli/cmd/grobase@main
   # without Go:
   git clone https://github.com/Univers42/grobase-cli && cd grobase-cli
   docker run --rm -v "$PWD":/src -w /src -e CGO_ENABLED=0 golang:1.26.9 \
       go build -buildvcs=false -o /src/grobase ./cmd/grobase
   install -m 0755 grobase ~/.local/bin/grobase
   ```

2. **Bring the three values** from the host's secrets file (`GROBASE_EMAIL`,
   `GROBASE_PASSWORD`, `GROBASE_ANON_KEY`) through your password manager. Never
   paste them into a chat, a ticket or a shell history.

3. **One context for every route**, in the order to try them (the first that
   answers is used; one that does not costs 2 s, `GROBASE_PROBE_TIMEOUT`):

   ```sh
   grobase context add b2b \
     --url https://192.168.1.132:8443 \
     --url https://dlesieur42-1.tail10f2ea.ts.net:8443 \
     --url https://100.94.119.97:8443 \
     --pin-sha256 28e27b66a18a90d701a406fbb479c071a26d08cc2fe250a7bb329d327e780c7a \
     --yes
   ```

   Off the tailnet, add the public route as its own context, because its
   certificate is different:
   `grobase context add b2b-public --url https://dlesieur42-1.tail10f2ea.ts.net --yes`.

4. **First connection with the account** (anon key first, then the password,
   both on stdin):

   ```sh
   read -rs -p 'anon key: ' AK; echo; read -rs -p 'password: ' PW; echo
   printf '%s\n%s\n' "$AK" "$PW" | grobase login --email cli-firstconnect-130748@example.com \
       --anon-key-stdin --password-stdin
   unset AK PW
   grobase whoami          # user {"email":"cli-firstconnect-130748@example.com", …}
   grobase doctor          # the reachability row names the URL in use
   ```

   A new account instead: `grobase signup --email you@example.com
   --anon-key-stdin --password-stdin` (the VM confirms emails itself, so signup
   also signs you in).

5. **Reset a forgotten password** (admin; needs the service-role key, which
   stays on the VM). Use a throwaway config so the admin key is not left in
   your store:

   ```sh
   export GROBASE_CONFIG=$(mktemp -d)/config.json GROBASE_PASSPHRASE=$(head -c 24 /dev/urandom | base64)
   grobase context add adm --url https://100.94.119.97:8443 --yes \
       --pin-sha256 28e27b66a18a90d701a406fbb479c071a26d08cc2fe250a7bb329d327e780c7a
   ssh -p 4242 dlesieur@100.94.119.97 'sed -n "s/^SERVICE_ROLE_KEY=//p" /opt/grobase/.env' |
       grobase login --service-role --credential-store file
   read -rs -p 'new password: ' PW; echo
   printf '%s\n' "$PW" | grobase users update cli-firstconnect-130748@example.com \
       --password-stdin --credential-store file
   rm -rf "$(dirname "$GROBASE_CONFIG")"; unset GROBASE_CONFIG GROBASE_PASSPHRASE PW
   ```

   Passing an email to `users update` needs CLI `59d2393` or later. Before it
   the email went into the URL unchanged and the WAF answered 403.

## 5. ssh into the VM from the other device

Once, from a device on the tailnet (or on the LAN, `192.168.1.132`):

```sh
ssh-keygen -t ed25519                                  # if the device has no key
ssh-copy-id -p 4242 dlesieur@100.94.119.97             # asks the VM password once
ssh -p 4242 dlesieur@100.94.119.97 true && echo key-ok
```

In Tailscale userspace mode ssh needs the proxy too:
`ssh -o ProxyCommand='nc -X 5 -x 127.0.0.1:1055 %h %p' -p 4242 dlesieur@100.94.119.97`.

## 6. Test every route in one command

```sh
scripts/check-routes.sh                                          # reach + TLS
GROBASE_EMAIL=… GROBASE_PASSWORD=… GROBASE_ANON_KEY=… scripts/check-routes.sh   # + login, whoami
ROUTES="tailnet-ip magicdns funnel" scripts/check-routes.sh      # a subset
```

It uses a throwaway config, so your contexts and stored login are not touched.
It prints one line per route (`PASS`, `FAIL` with the reason, or `SKIP`) and
exits 0 when at least one passed. Expected results by where the device is:

| Device | Should PASS | Expected FAIL |
|---|---|---|
| Same wifi, no Tailscale | `lan-ip`, `ssh-lan`, `funnel`; `lan-name` with the hosts entry | `tailnet-ip`, `magicdns`, `ssh-tailnet` |
| Elsewhere, Tailscale | `tailnet-ip`, `magicdns`, `ssh-tailnet`, `funnel` | `lan-*`, `ssh-lan` |
| Elsewhere, no Tailscale | `funnel` | everything else |

The ssh check reads the banner over a plain socket: it ignores `HTTPS_PROXY`
and does not prove your key works (§5 does).

## 7. Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `magicdns` / `funnel` FAIL with `lookup … i/o timeout` on a machine running hide | hide hijacked every DNS query, including Tailscale's resolver `100.100.100.100` (TCP and UDP port 53 timed out while `http://100.100.100.100` answered 200) | hide branch `fix/magicdns` (`209f390`) exempts that resolver; install it (born2root report). Until then use the IP routes |
| `tls FAIL connect …: dial tcp: lookup …` only with `HTTPS_PROXY` set | CLI older than `7cb3ba9` probes the certificate without the proxy | update the CLI |
| LAN route FAILs with the CA file | the certificate has no SAN for `192.168.1.132` | use the pin, or the hosts name with the CA |
| LAN route times out from a wifi device, works from the host | the wifi isolates clients (guest network, campus wifi) | Tailscale, or Funnel |
| `doctor` `tls` FAIL with two fingerprints | the leaf was re-issued | check `openssl x509 -noout -fingerprint -sha256 -in /opt/grobase/certs/localhost.pem` on the VM, then `grobase context pin b2b` |
| `users update <email>` 403 | CLI older than `59d2393` | update, or pass the id from `grobase users get <email>` |
| `context show` names a URL you did not expect | it prints the first stored URL | `grobase context env b2b` or `doctor` show the one in use |
| `error obtaining VCS status` / `go.mod requires go >= 1.26.9` when building | container build | `-buildvcs=false`; `golang:1.26.9` |
| every tailnet route FAILs and `dlesieur42-1` is missing from `tailscale status` | the VM's node key expired (it expires 2027-04-08), or the node was removed: born2root mints its auth keys ephemeral, and Tailscale deletes an ephemeral node some time after it goes offline | on the host, born2root `make tailnet`. The VM may come back with another 100.x address or name: run `scripts/check-routes.sh` and update your context and `.env.grobase-link`. Disabling key expiry for the VM in the admin console removes the first cause |

## 8. What was run

All on 2026-10-12, with the static CLI built from `7cb3ba9` and
`scripts/check-routes.sh`, as the account above (login and whoami on every
HTTPS route).

The test devices were stand-ins on the host. **A** is a container on Docker's
bridge network: its packets reach `192.168.1.132` from outside the host's
loopback, through the host firewall (ufw, default DROP), as a LAN device's do.
**B** is a separate, ephemeral Tailscale node (`tailscale/tailscale:v1.102.5`,
userspace mode), forced onto Tailscale's relays (`TS_DEBUG_ALWAYS_USE_DERP`),
which is the path a device on another network takes when no direct path exists.
**C** is a container that left through the public internet (direct, not Tor)
and resolved the name through public DNS (`1.1.1.1`), as a device without
Tailscale does.

| # | Device | Route | Result |
|---|---|---|---|
| A | bridge container | `lan-ip` | PASS, 42 ms, login + whoami |
| A | bridge container | `ssh-lan` | PASS, banner `SSH-2.0-OpenSSH_10.0p2` |
| A | bridge container | `tailnet-ip`, `ssh-tailnet` | PASS (forwarded through the host's node) |
| B | tailnet node over DERP | path to the VM | `pong from dlesieur42-1 via DERP(mad) in 8ms` |
| B | tailnet node over DERP | MagicDNS | `dlesieur42-1.tail10f2ea.ts.net` → `100.94.119.97` |
| B | tailnet node over DERP | `tailnet-ip` | PASS, 35 ms, login + whoami |
| B | tailnet node over DERP | `magicdns` | PASS, 29 ms, login + whoami |
| B | tailnet node over DERP | ssh | key login, `ssh ok: dlesieur42 dlesieur` |
| B | same node, CLI `2ffd17d` | `magicdns` | FAIL on `tls` (the probe ignored the proxy; fixed in `7cb3ba9`) |
| C | public internet | `funnel` | PASS, 1118 ms, login + whoami; name resolved to `176.58.90.145` |
| C | same, after WAF rule 10130 | `funnel` + private paths | PASS, 289 ms, login + whoami; `/admin/v1/tenants`, `/admin/v1/keys`, `/meta/v1/tables`, `/realtime/v1/publish` 403 (rule 10130 in the WAF audit log), `/auth/v1/health` 401 from Kong |
| — | host | `lan-ip`, `tailnet-ip`, `ssh-*` | PASS; `magicdns`, `funnel` FAIL (hide's DNS, §7) |
| — | host, after the branch merges | every route but `lan-name` (no hosts entry) | PASS, login + whoami; `magicdns` and `funnel` too: the name resolved on `tailscale0` with hide's global mode on |

Not run:

- A physical second computer or phone, on this wifi or on another network.
  The stand-ins cover the host side of each path, not a wifi that isolates
  clients or a network that blocks Tailscale's relays.
- groot's `make link` from another device. The 2026-10-11 runbook records it
  from the host over the Tailscale IP: `groot/wiki/runbooks/grobase-link.md`.
- `lan-name` from a device (only `curl --resolve` on the host).

## 9. groot

groot's frontends reach grobase through a link. On the other device:

```sh
git clone --recursive git@github.com:Univers42/groot.git groot && cd groot
```

groot's `main` holds the link. It pins grobase and osionos at their own `main`,
so the submodule sync in plain `make all` keeps the code the link needs.
`make all SKIP_SYNC=1` builds exactly the pinned commits; use it to reproduce
a run. One pin is ahead of its `main`: opposite-osiris (the portal). The sync
moves it to its `main`, which changes the portal, not the link.

`.env.grobase-link` (git-ignored), one of:

```sh
GROBASE_TARGET=ssh://dlesieur@100.94.119.97:4242   # needs the ssh key (§5); fetches the VM's secrets itself
GROBASE_TARGET=https://100.94.119.97:8443          # direct; then also:
GROBASE_CA_FILE=$HOME/grobase-ca.pem               # copied from the VM, fingerprint checked
```

```sh
make link && make link-frontends-up && make link-verify    # "verify: N frontends reach grobase …"
```

Copy the CA for direct mode, and compare its fingerprint with the VM's:

```sh
scp -P 4242 dlesieur@100.94.119.97:/opt/grobase/certs/track-binocle-local-ca.pem ~/grobase-ca.pem
openssl x509 -noout -fingerprint -sha256 -in ~/grobase-ca.pem
ssh -p 4242 dlesieur@100.94.119.97 openssl x509 -noout -fingerprint -sha256 \
    -in /opt/grobase/certs/track-binocle-local-ca.pem
```

## 10. Migrations and the host, from the other device

Both need a CLI built with `db migrate` and `host`, and the VM's grobase at
`9f26e792` or later (data plane token `8fa693d2`, WAF rule 10120). Use the throwaway admin
config from §4 step 5, with `--ssh` added to the context:

```sh
grobase context add adm --url https://100.94.119.97:8443 --yes --ssh b2b \
    --pin-sha256 28e27b66a18a90d701a406fbb479c071a26d08cc2fe250a7bb329d327e780c7a
grobase db list --tenant agency --credential-store file           # 7 mounts on 2026-10-12
grobase db migrate --tenant agency --mount pg-commerce ./migrations --credential-store file
```

`./migrations` holds `0001_create.sql`, `0002_seed.sql`, ...; each file is one
migration, sent whole. The second run of the same directory prints `skipped`
for every file. A changed file needs a new name: the server keys on the name.

`b2b` is an `~/.ssh/config` alias for the VM (§5). Then:

```sh
grobase host ps                  # every container of the mini-baas project
grobase host health              # exit 1 names each unhealthy / crash-looping / failed one
grobase host logs waf --since 10m --tail 100
grobase host restart mailpit     # asks; --yes to skip
```

Measured on 2026-10-12 against the baas VM, from this host over the tailnet IP:

| Step | Result |
|---|---|
| `db migrate` pg-commerce, run 1 / run 2 | `applied` ×2 / `skipped` ×2, exit 0 |
| `db migrate` mysql-ops, run 1 / run 2 | `applied` ×2 / `skipped` ×2, exit 0 |
| rows written (`it's here; -- not a comment`, `<b>bold</b> ../../etc/passwd`) | identical in Postgres `commerce` and MySQL `ops` |
| a file with a syntax error after them | `400 … syntax error at or near "CREAT"`, exit 1, no marker left |
| a MongoDB mount | `422 engine 'mongodb' does not support requested capability 'ddl'`, exit 1 |
| `--mount pg-comerce` (typo) | exit 4, lists the tenant's 7 mounts |
| `db list` without `--tenant` | `switched off on this deployment` (TENANT_SELFSERVE_ENABLED is off) |
| `--debug` migrate trace | 0 copies of the service-role key, 8 masks |
| `host health` | `40 containers of mini-baas: none unhealthy, crash-looping or failed` |
| `host logs waf` | the raw log held the service-role key on 28 lines; the CLI printed it on 0 |
| `host restart mailpit` without a terminal / with `--yes` | exit 2 `pass --yes to confirm` / restarted, healthy 6 s later |

The test tables were dropped afterwards by one more migration
(`20261012_grobase_cli_live_test_cleanup`), whose marker is the only trace left.
