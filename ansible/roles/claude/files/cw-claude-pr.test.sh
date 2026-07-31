#!/usr/bin/env bash
# Unit test for cw-claude-pr's pure logic: label membership validation and the
# issue-link line. The gh calls (label list, pr create) are integration.
set -uo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=cw-claude-pr
source "$here/cw-claude-pr"

fail=0
check() { # <expected> <actual> <label>
  if [ "$2" = "$1" ]; then
    echo "ok: $3"
  else
    echo "FAIL: $3 — expected '$1', got '$2'"
    fail=1
  fi
}

labels=$'bug\ndocumentation\nenhancement\narea:security\np1'

# label_ok: exact-line membership, not a substring.
check yes "$(label_ok bug "$labels" && echo yes || echo no)"          "known label -> ok"
check yes "$(label_ok area:security "$labels" && echo yes || echo no)" "namespaced label -> ok"
check no  "$(label_ok area:nope "$labels" && echo yes || echo no)"     "unknown label -> not ok"
check no  "$(label_ok bu "$labels" && echo yes || echo no)"            "substring is not a match"
check no  "$(label_ok '' "$labels" && echo yes || echo no)"           "empty label -> not ok"

# issue_ref_line: Closes wins if both, Refs otherwise, empty if neither.
check "Closes #5" "$(issue_ref_line 5 '')"  "closes -> Closes #N"
check "Refs #7"   "$(issue_ref_line '' 7)"  "refs -> Refs #N"
check ""          "$(issue_ref_line '' '')" "neither -> empty"

exit "$fail"
