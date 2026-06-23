#!/usr/bin/env bash
# Fail the commit if any file that is meant to be SOPS-encrypted is staged in
# plaintext. pre-commit passes the matched filenames as arguments (filtered by
# the `files:` regex in .pre-commit-config.yaml, which mirrors .sops.yaml).
#
# A SOPS-encrypted file always contains ENC[...] value markers; a plaintext one
# does not. New/empty secret files therefore correctly fail until encrypted.
set -euo pipefail

status=0
for f in "$@"; do
  [ -f "$f" ] || continue
  if ! grep -q 'ENC\[' "$f"; then
    echo "ERROR: '$f' matches a SOPS path but is not encrypted (no ENC[...] markers)."
    echo "       Encrypt it first:  sops -e -i '$f'"
    status=1
  fi
done
exit "$status"
