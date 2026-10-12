#!/usr/bin/env bash
# check-routes.sh: from the device it runs on, which routes to the b2b grobase answer,
# proven with the grobase CLI itself. Each HTTPS route gets a throwaway context in a
# temporary config, so your own contexts and stored credentials are never touched.
#
#   scripts/check-routes.sh                    reachability + TLS, every route
#   ROUTES="tailnet-ip magicdns" scripts/check-routes.sh      only those
#   GROBASE_EMAIL=… GROBASE_PASSWORD=… GROBASE_ANON_KEY=… scripts/check-routes.sh
#                                              the same, plus login and whoami per route
#
# An HTTPS route passes when `grobase doctor` reports reachability and TLS PASS (and, with
# the three credentials, `login` and `whoami` succeed). An ssh route passes when port 4242
# answers with an SSH banner. Exit 0 when at least one route passed.
#
# Caveat: a PASS proves this device's path at this moment, nothing about another device;
# the LAN routes are expected to FAIL off the LAN. The ssh check opens a plain TCP socket,
# so it ignores HTTPS_PROXY (Tailscale userspace mode) and only reads the banner: it does
# not prove your key is accepted. Needs bash, timeout (coreutils) and the grobase CLI.
set -u

PIN=${GROBASE_PIN:-28e27b66a18a90d701a406fbb479c071a26d08cc2fe250a7bb329d327e780c7a}
LAN_IP=${GROBASE_LAN_IP:-192.168.1.132}
LAN_NAME=${GROBASE_LAN_NAME:-grobase.track-binocle.test}
TS_IP=${GROBASE_TS_IP:-100.94.119.97}
TS_NAME=${GROBASE_TS_NAME:-dlesieur42-1.tail10f2ea.ts.net}
ROUTES=${ROUTES:-lan-ip lan-name tailnet-ip magicdns funnel ssh-lan ssh-tailnet}
GROBASE=${GROBASE_BIN:-grobase}
STEP=${GROBASE_STEP_TIMEOUT:-30}

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
export GROBASE_CONFIG="$work/config.json"
GROBASE_PASSPHRASE=$(head -c 24 /dev/urandom | base64)
export GROBASE_PASSPHRASE
passed=0

g() { timeout "$STEP" "$GROBASE" --credential-store file --timeout 10s "$@"; }

report() {
    printf '%-4s  %-11s  %-46s  %s\n' "$1" "$2" "$3" "$4"
    if [ "$1" = PASS ]; then passed=$((passed + 1)); fi
}

# https_route NAME URL PIN: PIN "-" trusts the system roots (Funnel's public certificate).
https_route() {
    local name=$1 url=$2 pin=$3 doc detail
    local add=(context add "$name" --url "$url" --yes)
    if [ "$pin" != - ]; then add+=(--pin-sha256 "$pin"); fi
    if ! g "${add[@]}" >"$work/$name.add" 2>&1; then
        report FAIL "$name" "$url" "context add: $(tail -n1 "$work/$name.add")"
        return
    fi
    doc=$(g --context "$name" doctor 2>&1)
    if ! grep -Eq '^dns/reachability +PASS' <<<"$doc" || ! grep -Eq '^tls +PASS' <<<"$doc"; then
        detail=$(grep -E '^(dns/reachability|tls) ' <<<"$doc" | grep -v PASS | head -n1 | tr -s ' ')
        [ -n "$detail" ] || detail=$(tail -n1 <<<"$doc")
        [ -n "$detail" ] || detail="doctor gave no answer within ${STEP}s"
        report FAIL "$name" "$url" "$detail"
        return
    fi
    detail="reach+tls $(grep -oE 'in [0-9]+ms' <<<"$doc" | head -n1)"
    if [ -n "${GROBASE_EMAIL:-}" ] && [ -n "${GROBASE_PASSWORD:-}" ] && [ -n "${GROBASE_ANON_KEY:-}" ]; then
        if ! printf '%s\n%s\n' "$GROBASE_ANON_KEY" "$GROBASE_PASSWORD" |
            g --context "$name" login --email "$GROBASE_EMAIL" --anon-key-stdin --password-stdin \
                >/dev/null 2>"$work/$name.login"; then
            report FAIL "$name" "$url" "$detail, login: $(tail -n1 "$work/$name.login")"
            return
        fi
        if ! g --context "$name" whoami 2>/dev/null | grep -qF "\"email\":\"$GROBASE_EMAIL\""; then
            report FAIL "$name" "$url" "$detail, login ok but whoami does not name $GROBASE_EMAIL"
            return
        fi
        detail="$detail, login+whoami as $GROBASE_EMAIL"
    fi
    report PASS "$name" "$url" "$detail"
}

ssh_route() {
    local name=$1 host=$2 banner
    # shellcheck disable=SC2016 # $1 and $line belong to the inner bash -c, not to this shell
    banner=$(timeout 8 bash -c 'exec 3<>"/dev/tcp/$1/4242" && IFS= read -r -t 6 line <&3 && printf %s "$line"' _ "$host" 2>/dev/null | tr -d '\r')
    case "$banner" in
    SSH-*) report PASS "$name" "ssh -p 4242 $host" "banner ${banner%% *}" ;;
    *) report FAIL "$name" "ssh -p 4242 $host" "no SSH banner within 8 s" ;;
    esac
}

# lan_name_resolves: the LAN name exists only through a hosts-file entry.
lan_name_resolves() {
    ! command -v getent >/dev/null 2>&1 || getent hosts "$LAN_NAME" >/dev/null 2>&1
}

printf '%-4s  %-11s  %-46s  %s\n' RES ROUTE TARGET DETAIL
for route in $ROUTES; do
    case "$route" in
    lan-ip) https_route lan-ip "https://$LAN_IP:8443" "$PIN" ;;
    lan-name)
        if lan_name_resolves; then
            https_route lan-name "https://$LAN_NAME:8443" "$PIN"
        else
            report SKIP lan-name "https://$LAN_NAME:8443" "add '$LAN_IP $LAN_NAME' to the hosts file"
        fi
        ;;
    tailnet-ip) https_route tailnet-ip "https://$TS_IP:8443" "$PIN" ;;
    magicdns) https_route magicdns "https://$TS_NAME:8443" "$PIN" ;;
    funnel) https_route funnel "https://$TS_NAME" - ;;
    ssh-lan) ssh_route ssh-lan "$LAN_IP" ;;
    ssh-tailnet) ssh_route ssh-tailnet "$TS_IP" ;;
    *) report SKIP "$route" - "unknown route" ;;
    esac
done
[ "$passed" -gt 0 ]
