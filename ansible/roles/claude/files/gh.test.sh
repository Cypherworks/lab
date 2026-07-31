#!/usr/bin/env bash
# Unit test for the gh wrapper: mints a fresh token, delegates to the real gh
# with args unchanged. Stubs cw-claude-token and the real gh (both boundaries).
set -uo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

fail=0
check() { # <expected> <actual> <label>
  if [ "$2" = "$1" ]; then
    echo "ok: $3"
  else
    echo "FAIL: $3 — expected '$1', got '$2'"
    fail=1
  fi
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
printf '#!/usr/bin/env bash\necho FRESHTOKEN\n' > "$tmp/cw-claude-token"
# shellcheck disable=SC2016  # $GH_TOKEN/$* are literal; the stub expands them
printf '#!/usr/bin/env bash\necho "token=$GH_TOKEN args=$*"\n' > "$tmp/realgh"
chmod +x "$tmp/cw-claude-token" "$tmp/realgh"

out=$(PATH="$tmp:$PATH" CW_CLAUDE_REAL_GH="$tmp/realgh" bash "$here/gh" pr view 5 --json state)
check "token=FRESHTOKEN args=pr view 5 --json state" "$out" \
  "fresh token exported + args delegated to the real gh"

exit "$fail"
