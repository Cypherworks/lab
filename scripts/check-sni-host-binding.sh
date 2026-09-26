#!/usr/bin/env bash
# Check that an HTTPS ingress refuses requests whose Host header doesn't
# match the TLS SNI. Use it on any path that forwards one SNI to a wildcard
# site that routes by Host (an L4 SNI passthrough in front of Caddy, say).
# Without SNI==Host enforcement a client presents the one forwarded SNI,
# then asks for any other vhost by Host header.
#
# Usage:
#   ./check-sni-host-binding.sh --ip <ingress-ip> --sni <name> <host>...
#
#   --ip    address to connect to (the public relay). Bypasses local DNS,
#           so the test takes the real external path even from inside.
#   --sni   the hostname the ingress forwards; sent as the TLS SNI on
#           every request.
#   <host>  vhosts that must NOT be served over that SNI.
#
# Passes (exit 0) only when:
#   - the control request (Host == SNI) gets an HTTP response other than
#     421, so the path is alive and a timeout can't pass as "blocked"; and
#   - every <host> request gets 421 Misdirected Request.
# Exit 1 on any failure, 2 on usage errors. Needs: curl.
set -euo pipefail

die() { echo "error: $*" >&2; exit 2; }

ip="" sni=""
hosts=()
while [ $# -gt 0 ]; do
  case "$1" in
    --ip)  ip="${2:-}"; shift 2 ;;
    --sni) sni="${2:-}"; shift 2 ;;
    -h|--help) sed -n '2,21p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) die "unknown option: $1" ;;
    *)  hosts+=("$1"); shift ;;
  esac
done
[ -n "$ip" ] || die "--ip is required"
[ -n "$sni" ] || die "--sni is required"
[ "${#hosts[@]}" -gt 0 ] || die "give at least one <host> to test"
command -v curl >/dev/null || die "curl is not installed"

# HTTP status for Host=$1 over SNI=$sni at $ip; 000 = no HTTP response.
status_for() {
  curl --http1.1 -sS -o /dev/null -w '%{http_code}' \
    --connect-timeout 5 --max-time 15 \
    --resolve "$sni:443:$ip" -H "Host: $1" "https://$sni/" 2>/dev/null || true
}

failed=0

control="$(status_for "$sni")"
if [ "$control" = "000" ] || [ "$control" = "421" ]; then
  echo "FAIL  control  Host=$sni -> $control (path not answering)"
  exit 1
fi
echo "ok    control  Host=$sni -> $control"

for h in "${hosts[@]}"; do
  code="$(status_for "$h")"
  if [ "$code" = "421" ]; then
    echo "ok    pivot    Host=$h -> 421"
  else
    echo "FAIL  pivot    Host=$h -> $code (served over SNI $sni)"
    failed=1
  fi
done
exit "$failed"
