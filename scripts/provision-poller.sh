#!/usr/bin/env bash
# provision-poller.sh — the headless deploy "screen". The bare-metal hosts have
# no display, so success = the host appears on the network and answers SSH at its
# planned IP. This waits for each host, runs a smoke check, and prints pass/fail.
#
# Usage:
#   provision-poller.sh [--user ansible] [--timeout 600] [--interval 10] \
#       pi-dns-1=10.200.20.11 pi-dns-2=10.200.20.12
set -euo pipefail

USER_=ansible TIMEOUT=600 INTERVAL=10
hosts=()
while [ $# -gt 0 ]; do
  case "$1" in
    --user) USER_=$2; shift 2 ;;
    --timeout) TIMEOUT=$2; shift 2 ;;
    --interval) INTERVAL=$2; shift 2 ;;
    -h|--help) sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *=*) hosts+=("$1"); shift ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done
[ ${#hosts[@]} -gt 0 ] || { echo "no hosts given (name=ip ...)" >&2; exit 2; }

ssh_opts=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=5)
rc=0
printf '%-14s %-16s %s\n' "HOST" "IP" "RESULT"
for spec in "${hosts[@]}"; do
  name=${spec%%=*}; ip=${spec#*=}
  printf '%-14s %-16s ' "$name" "$ip"
  deadline=$(( $(date +%s) + TIMEOUT ))
  up=0
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if nc -z -w 5 "$ip" 22 2>/dev/null; then up=1; break; fi
    sleep "$INTERVAL"
  done
  if [ "$up" -ne 1 ]; then
    echo "FAIL (no SSH within ${TIMEOUT}s)"; rc=1; continue
  fi
  if out=$(ssh "${ssh_opts[@]}" "${USER_}@${ip}" 'hostname; . /etc/os-release; printf "%s" "$VERSION"' 2>/dev/null); then
    echo "PASS ($(echo "$out" | paste -sd' ' -))"
  else
    echo "FAIL (SSH open, key/login rejected)"; rc=1
  fi
done
exit "$rc"
