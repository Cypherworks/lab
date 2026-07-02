#!/usr/bin/env bash
# Sign your SSH public key with the OpenBao SSH CA so you can `ssh <you>@<host>` to
# any lab host that trusts the CA (the ssh_ca_trust role). The cert principal is
# locked to your own OpenBao/Authentik identity server-side, so you can only mint a
# cert for your own username — that's the audit guarantee.
#
# Usage:
#   BAO_ADDR=https://bao.cypherworks.co.uk ./bao-ssh-sign.sh <authentik-username> [pubkey]
#
#   <authentik-username>  your Authentik login (e.g. loliver) — must match your
#                         identity or OpenBao rejects the request.
#   [pubkey]              public key to sign (default: ~/.ssh/id_ed25519.pub).
#
# Writes the cert next to the key as <key>-cert.pub, which ssh loads automatically —
# so afterwards `ssh <you>@<host>` just works until the cert expires (1h).
#
# Needs: the `bao` CLI and a browser for the one-time OIDC login. Re-run when the
# cert expires (or wrap in a shell alias).
set -euo pipefail

BAO_ADDR="${BAO_ADDR:-https://bao.cypherworks.co.uk}"
BAO_SSH_MOUNT="${BAO_SSH_MOUNT:-ssh-client-signer}"
BAO_SSH_ROLE="${BAO_SSH_ROLE:-user}"
export BAO_ADDR

die() { echo "error: $*" >&2; exit 1; }

[ $# -ge 1 ] || die "usage: BAO_ADDR=… $0 <authentik-username> [pubkey]"
principal="$1"
pubkey="${2:-$HOME/.ssh/id_ed25519.pub}"
[ -r "$pubkey" ] || die "public key not found: $pubkey (pass one as arg 2)"
command -v bao >/dev/null || die "the 'bao' CLI is not installed"

# The signed cert lands beside the key: id_ed25519.pub -> id_ed25519-cert.pub.
cert="${pubkey%.pub}-cert.pub"

# Log in via Authentik OIDC only if there's no valid token already (opens a browser).
if ! bao token lookup >/dev/null 2>&1; then
  echo "Logging in to OpenBao via Authentik (browser)…" >&2
  bao login -method=oidc >/dev/null
fi

echo "Signing $pubkey for principal '$principal' (role $BAO_SSH_ROLE)…" >&2
bao write -field=signed_key "${BAO_SSH_MOUNT}/sign/${BAO_SSH_ROLE}" \
  public_key=@"$pubkey" valid_principals="$principal" > "$cert"

chmod 0644 "$cert"
echo "Wrote $cert" >&2
ssh-keygen -L -f "$cert" | sed -n '1,12p'
echo >&2
echo "Done — 'ssh ${principal}@<host>' will use it until it expires." >&2
