#!/usr/bin/env bash
# Unit test for cw-claude-push's pure logic: remote-URL parsing and the diff to
# fileChanges shaping (base64 additions, path deletions), on a throwaway repo.
# The API calls are integration, tested on deploy.
set -uo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=cw-claude-push
source "$here/cw-claude-push"

fail=0
check() { # <expected> <actual> <label>
  if [ "$2" = "$1" ]; then
    echo "ok: $3"
  else
    echo "FAIL: $3 — expected '$1', got '$2'"
    fail=1
  fi
}

# slug_from_url: https/ssh, with and without .git.
check octo-org/widget "$(slug_from_url https://github.com/octo-org/widget.git)" "https + .git"
check octo-org/widget "$(slug_from_url https://github.com/octo-org/widget)"     "https no .git"
check octo-org/widget "$(slug_from_url git@github.com:octo-org/widget.git)"     "ssh form"

# changes_json: build a real repo, take a base commit, then add/modify/delete.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
out=$(
  cd "$tmp" || exit 1
  git init -q .
  git config user.email t@example.test
  git config user.name test
  git config commit.gpgsign false
  printf 'old\n' > keep.txt
  printf 'gone\n' > del.txt
  git add -A && git commit -qm base
  base=$(git rev-parse HEAD)
  printf 'new\n' > keep.txt       # modify
  printf 'hi\n'  > add.txt        # add
  git rm -q del.txt               # delete
  git add -A && git commit -qm head
  changes_json "$base" "$(git rev-parse HEAD)"
)

check 2 "$(jq '.additions | length' <<<"$out")" "two additions (added + modified)"
check 1 "$(jq '.deletions | length' <<<"$out")" "one deletion"
check add.txt "$(jq -r '.additions[] | select(.path=="add.txt") | .path' <<<"$out")" "add.txt is an addition"
check del.txt "$(jq -r '.deletions[0].path' <<<"$out")" "del.txt is the deletion"
added_content=$(jq -r '.additions[] | select(.path=="add.txt") | .contents' <<<"$out" | base64 -d)
check hi "$(printf '%s' "$added_content" | tr -d '\n')" "addition contents base64 round-trip"

exit "$fail"
