#!/usr/bin/env bash
# Unit test for cw-claude-credential's host gate: the token is served only on a
# `get` for github.com, never for another host or a store/erase op.
set -uo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=cw-claude-credential
source "$here/cw-claude-credential"

fail=0
check() { # <expected> <actual> <label>
  if [ "$2" = "$1" ]; then
    echo "ok: $3"
  else
    echo "FAIL: $3 — expected '$1', got '$2'"
    fail=1
  fi
}

check yes "$(should_serve get github.com)"     "get + github.com -> serve"
check no  "$(should_serve get evil.example)"   "get + other host -> no (no token leak)"
check no  "$(should_serve store github.com)"   "store -> no"
check no  "$(should_serve erase github.com)"   "erase -> no"
check no  "$(should_serve get '')"             "get + empty host -> no"

exit "$fail"
