#!/usr/bin/env bash
# flash-x86.sh — build a FULLY AUTOMATED Ubuntu 24.04 autoinstall image for a
# HEADLESS x86 lab host (ThinkCentre / Ryzen). Remasters the live-server ISO so
# it boots straight into an unattended install onto the host's planned static IP:
# plug in, power on, wait for it to come online — no console, no keypress.
#
# It bakes `autoinstall` into the GRUB default entry and embeds a NoCloud seed,
# then repacks preserving BIOS+UEFI boot (xorriso self-derives the boot layout
# from the source ISO, so nothing is hand-coded). Output is either an .iso
# (--output, for inspection) or written straight to a USB (--device).
#
# macOS only. Needs xorriso (`brew install xorriso`). --device is destructive.
# Per-host data is passed in by the caller — this is mechanism; site data lives
# in homelab-deploy.
#
# Usage:
#   flash-x86.sh --hostname node-tc1 --ip 10.200.20.21 --gateway 10.200.20.1 \
#                --pubkey ~/.ssh/id_ed25519.pub \
#                --iso ~/Downloads/ubuntu-24.04.4-live-server-amd64.iso \
#                --device /dev/disk5            # or: --output node-tc1.iso
#                [--prefix 24] [--dns 1.1.1.1] [--user ansible] [--match 'en*']
set -euo pipefail

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
usage() { sed -n '2,26p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-0}"; }

HOSTNAME='' IP='' GATEWAY='' PUBKEY='' ISO='' DEVICE='' OUTPUT=''
PREFIX=24 DNS=1.1.1.1 USERNAME=ansible MATCH='en*'
while [ $# -gt 0 ]; do
  case "$1" in
    --hostname) HOSTNAME=$2; shift 2 ;;
    --ip) IP=$2; shift 2 ;;
    --gateway) GATEWAY=$2; shift 2 ;;
    --pubkey) PUBKEY=$2; shift 2 ;;
    --iso) ISO=$2; shift 2 ;;
    --device) DEVICE=$2; shift 2 ;;
    --output) OUTPUT=$2; shift 2 ;;
    --prefix) PREFIX=$2; shift 2 ;;
    --dns) DNS=$2; shift 2 ;;
    --user) USERNAME=$2; shift 2 ;;
    --match) MATCH=$2; shift 2 ;;
    -h|--help) usage 0 ;;
    *) die "unknown arg: $1" ;;
  esac
done

[ "$(uname)" = "Darwin" ] || die "this script targets macOS"
command -v xorriso >/dev/null || die "xorriso not found — run: brew install xorriso"
for v in HOSTNAME IP GATEWAY PUBKEY ISO; do [ -n "${!v}" ] || die "missing --${v,,}"; done
[ -f "$ISO" ] || die "iso not found: $ISO"
[ -f "$PUBKEY" ] || die "pubkey not found: $PUBKEY"
[ -n "$DEVICE" ] || [ -n "$OUTPUT" ] || die "need --device or --output"
[ -z "$DEVICE" ] || [ -z "$OUTPUT" ] || die "use --device OR --output, not both"
KEY=$(cat "$PUBKEY")

WORK=$(mktemp -d) || die "mktemp failed"
OUT=${OUTPUT:-$(mktemp -u "${TMPDIR:-/tmp}/${HOSTNAME}.XXXX.iso")}
cleanup() { rm -rf "$WORK"; [ -n "$OUTPUT" ] || rm -f "$OUT"; }
trap cleanup EXIT

echo "Extracting $ISO ..."
xorriso -osirrox on -indev "$ISO" -extract / "$WORK" 2>/dev/null
chmod -R u+w "$WORK"

# NoCloud seed (read by the installer at /cdrom/nocloud/).
mkdir -p "$WORK/nocloud"
cat > "$WORK/nocloud/meta-data" <<EOF
instance-id: ${HOSTNAME}
local-hostname: ${HOSTNAME}
EOF
# Key-only ansible account (locked password) + NOPASSWD sudo, static IP on the
# first wired NIC (glob-matched, since x86 names vary). base/CIS take over after.
cat > "$WORK/nocloud/user-data" <<EOF
#cloud-config
autoinstall:
  version: 1
  locale: en_GB.UTF-8
  keyboard:
    layout: gb
  identity:
    hostname: ${HOSTNAME}
    username: ${USERNAME}
    password: "!"
  ssh:
    install-server: true
    allow-pw: false
    authorized-keys:
      - "${KEY}"
  network:
    version: 2
    ethernets:
      lab0:
        match:
          name: "${MATCH}"
        dhcp4: false
        dhcp6: false
        addresses:
          - ${IP}/${PREFIX}
        routes:
          - to: default
            via: ${GATEWAY}
        nameservers:
          addresses: [${DNS}]
  storage:
    layout:
      name: lvm
  packages:
    - python3
  late-commands:
    - curtin in-target --target=/target -- usermod -aG sudo ${USERNAME}
    - "echo '${USERNAME} ALL=(ALL) NOPASSWD:ALL' > /target/etc/sudoers.d/90-${USERNAME}"
    - chmod 0440 /target/etc/sudoers.d/90-${USERNAME}
    - curtin in-target --target=/target -- passwd -l ${USERNAME}
EOF

# Boot the autoinstall entry immediately, unattended.
GRUB="$WORK/boot/grub/grub.cfg"
sed -i '' 's/^set timeout=.*/set timeout=1/' "$GRUB"
sed -i '' 's#/casper/vmlinuz  *---#/casper/vmlinuz autoinstall ds=nocloud\\;s=/cdrom/nocloud/ ---#' "$GRUB"

echo "Repacking ..."
# Self-derive the source ISO's boot layout (BIOS+UEFI) and reuse it verbatim.
# Flatten to one line: as_mkisofs reports one option per line, and eval would
# otherwise treat the newlines as command separators.
FLAGS=$(xorriso -indev "$ISO" -report_el_torito as_mkisofs 2>/dev/null | tr '\n' ' ')
if ! eval xorriso -as mkisofs "$FLAGS" -o "$OUT" "$WORK" >"$WORK.log" 2>&1; then
  tail -20 "$WORK.log"; die "repack failed"
fi
rm -f "$WORK.log"

if [ -n "$DEVICE" ]; then
  printf 'Write %s to %s (ERASES it)? [y/N] ' "$HOSTNAME" "$DEVICE"
  read -r ans; [ "$ans" = y ] || die "aborted"
  diskutil unmountDisk "$DEVICE" >/dev/null || true
  sudo dd if="$OUT" of="$DEVICE" bs=4m
  diskutil eject "$DEVICE" >/dev/null || true
  echo "Done. Plug into ${HOSTNAME}, power on; it autoinstalls onto ${IP}."
else
  echo "Wrote ${OUT} (${HOSTNAME} -> ${IP})."
fi
