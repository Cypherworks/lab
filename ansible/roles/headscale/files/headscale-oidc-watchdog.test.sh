#!/usr/bin/env bash
# Unit test for the headscale-oidc-watchdog decision logic. Sources the script
# (which must not run its main when sourced) and exercises decide() across the
# four issuer-reachable / headscale-degraded combinations.
set -uo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=headscale-oidc-watchdog.sh
source "$here/headscale-oidc-watchdog.sh"

fail=0
check() { # <expected> <actual> <label>
  if [ "$2" = "$1" ]; then
    echo "ok: $3"
  else
    echo "FAIL: $3 — expected '$1', got '$2'"
    fail=1
  fi
}

# Restart only when the issuer is back AND headscale came up OIDC-degraded.
check restart "$(decide yes yes)" "issuer reachable + headscale degraded -> restart"
check noop    "$(decide yes no)"  "issuer reachable + headscale healthy  -> noop"
check noop    "$(decide no yes)"  "issuer down + degraded -> noop (don't restart into failure)"
check noop    "$(decide no no)"   "issuer down + healthy  -> noop"

exit "$fail"
