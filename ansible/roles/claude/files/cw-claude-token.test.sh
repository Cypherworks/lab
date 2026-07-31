#!/usr/bin/env bash
# Unit test for cw-claude-token's pure logic: the freshness decision, base64url
# encoding, the JWT claim set, and a real RS256 sign/verify round-trip with a
# throwaway key. The API calls are integration, tested on deploy.
set -uo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=cw-claude-token
source "$here/cw-claude-token"

fail=0
check() { # <expected> <actual> <label>
  if [ "$2" = "$1" ]; then
    echo "ok: $3"
  else
    echo "FAIL: $3 — expected '$1', got '$2'"
    fail=1
  fi
}

# Reverse of b64url(), for decoding what jwt_sign produced.
b64url_decode() {
  local s="$1"
  local pad=$(( (4 - ${#s} % 4) % 4 ))
  while [ "$pad" -gt 0 ]; do s+="="; pad=$((pad - 1)); done
  printf '%s' "$s" | tr '_-' '/+' | openssl base64 -d -A
}

# token_fresh: re-mint once inside the skew window.
check fresh "$(token_fresh 2000 1000 300)" "expiry well ahead -> fresh"
check stale "$(token_fresh 1100 1000 300)" "expiry inside skew window -> stale"
check stale "$(token_fresh 990 1000 300)"  "already expired -> stale"
check fresh "$(token_fresh 1301 1000 300)" "just outside skew -> fresh"
check stale "$(token_fresh 1300 1000 300)" "exactly at skew boundary -> stale"

# b64url: no padding, URL alphabet.
check aGVsbG8 "$(printf 'hello' | b64url)" "b64url encodes and strips padding"
check Pj4-    "$(printf '>>>'   | b64url)" "b64url maps + to -"

# jwt_payload: iat 60s back, exp 9 min out, iss = app id.
payload=$(jwt_payload 4444760 1000000)
check 999940  "$(printf '%s' "$payload" | jq -r '.iat')" "iat is now-60"
check 1000540 "$(printf '%s' "$payload" | jq -r '.exp')" "exp is now+540"
check 4444760 "$(printf '%s' "$payload" | jq -r '.iss')" "iss is the app id"
check 600     "$(printf '%s' "$payload" | jq -r '.exp - .iat')" "exp-iat within the cap"

# jwt_sign: real RS256 round-trip with a throwaway key.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
openssl genrsa -out "$tmp/key.pem" 2048 >/dev/null 2>&1
openssl rsa -in "$tmp/key.pem" -pubout -out "$tmp/pub.pem" >/dev/null 2>&1

jwt=$(jwt_sign 4444760 "$tmp/key.pem" 1000000)
check 3 "$(printf '%s' "$jwt" | awk -F. '{print NF}')" "jwt has header.payload.signature"
check 4444760 "$(b64url_decode "$(printf '%s' "$jwt" | cut -d. -f2)" | jq -r '.iss')" "payload carries app id"

# The signature verifies over 'header.payload' with the matching public key.
signing_input=$(printf '%s' "$jwt" | cut -d. -f1-2)
b64url_decode "$(printf '%s' "$jwt" | cut -d. -f3)" > "$tmp/sig.bin"
if printf '%s' "$signing_input" | openssl dgst -sha256 -verify "$tmp/pub.pem" -signature "$tmp/sig.bin" >/dev/null 2>&1; then
  echo "ok: RS256 signature verifies with the public key"
else
  echo "FAIL: RS256 signature did not verify"
  fail=1
fi

exit "$fail"
