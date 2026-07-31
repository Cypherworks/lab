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

# End-to-end via a stub gh: proves draft + labels + reviewer + issue link.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/gh" <<'STUB'
#!/usr/bin/env bash
if [ "$1 $2" = "label list" ]; then
  printf 'documentation\narea:devsecops\np1\n'
elif [ "$1 $2" = "pr create" ]; then
  shift 2
  printf '%s\n' "$@"
fi
STUB
chmod +x "$tmp/gh"

out=$(PATH="$tmp:$PATH" CW_CLAUDE_REVIEWER=reviewbot \
  main --type documentation --area area:devsecops --priority p1 \
       --title t --body b --closes 5)
has() { printf '%s\n' "$out" | grep -qxF -- "$1"; }
check yes "$(has --draft && echo yes || echo no)"        "create call is a draft"
check yes "$(has documentation && echo yes || echo no)"  "type label passed"
check yes "$(has area:devsecops && echo yes || echo no)" "area label passed"
check yes "$(has p1 && echo yes || echo no)"             "priority label passed"
check yes "$(has reviewbot && echo yes || echo no)"      "reviewer/assignee passed"
check yes "$(printf '%s\n' "$out" | grep -qF 'Closes #5' && echo yes || echo no)" "body links issue"

# repo_has_area_labels: only when an area:* label exists.
check yes "$(repo_has_area_labels "$labels")"             "area:* present -> yes"
check no  "$(repo_has_area_labels $'bug\ndocumentation')" "no area:* -> no"

# A repo without area:* labels: --area not required; type-only create.
cat > "$tmp/gh" <<'STUB2'
#!/usr/bin/env bash
if [ "$1 $2" = "label list" ]; then
  printf 'bug\ndocumentation\nenhancement\n'
elif [ "$1 $2" = "pr create" ]; then
  shift 2; printf '%s\n' "$@"
fi
STUB2
chmod +x "$tmp/gh"
out2=$(PATH="$tmp:$PATH" main --type documentation --title t --body b)
has2() { printf '%s\n' "$out2" | grep -qxF -- "$1"; }
check yes "$(has2 --draft && echo yes || echo no)"       "no-area repo: still a draft"
check yes "$(has2 documentation && echo yes || echo no)" "no-area repo: type label passed"
check no  "$(printf '%s\n' "$out2" | grep -q '^area:' && echo yes || echo no)" "no-area repo: no area label"

exit "$fail"
