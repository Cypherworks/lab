#!/usr/bin/env bash
# flash-x86.sh — build ONE generic, fully-automated Ubuntu 24.04 autoinstall
# image for the headless x86 lab hosts (both ThinkCentres + the Ryzen). Remasters
# the live-server ISO so it boots straight into an unattended install: DHCP, the
# ansible user + SSH key, fresh per-install host keys. Plug in, power on, wait.
#
# Per-host identity is NOT baked in — it comes from a DHCP reservation (MAC -> IP,
# in terraform/unifi) plus the base role (hostname + permanent static netplan).
# So the same USB does every node; DHCP also makes the installer wait for the
# link to actually forward before apt runs (no configure_apt race on slow ports).
#
# macOS only. Needs xorriso (`brew install xorriso`). --device is destructive.
#
# Usage:
#   flash-x86.sh --pubkey ~/.ssh/id_ed25519.pub \
#                --iso ~/Downloads/ubuntu-24.04.4-live-server-amd64.iso \
#                --device /dev/disk5           # or: --output nodes.iso
#                [--user ansible]
set -euo pipefail

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
usage() { sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-0}"; }

PUBKEY='' ISO='' DEVICE='' OUTPUT='' USERNAME=ansible
while [ $# -gt 0 ]; do
  case "$1" in
    --pubkey) PUBKEY=$2; shift 2 ;;
    --iso) ISO=$2; shift 2 ;;
    --device) DEVICE=$2; shift 2 ;;
    --output) OUTPUT=$2; shift 2 ;;
    --user) USERNAME=$2; shift 2 ;;
    -h|--help) usage 0 ;;
    *) die "unknown arg: $1" ;;
  esac
done

[ "$(uname)" = "Darwin" ] || die "this script targets macOS"
command -v xorriso >/dev/null || die "xorriso not found — run: brew install xorriso"
for v in PUBKEY ISO; do [ -n "${!v}" ] || die "missing --${v,,}"; done
[ -f "$ISO" ] || die "iso not found: $ISO"
[ -f "$PUBKEY" ] || die "pubkey not found: $PUBKEY"
[ -n "$DEVICE" ] || [ -n "$OUTPUT" ] || die "need --device or --output"
[ -z "$DEVICE" ] || [ -z "$OUTPUT" ] || die "use --device OR --output, not both"
KEY=$(cat "$PUBKEY")

WORK=$(mktemp -d) || die "mktemp failed"
OUT=${OUTPUT:-$(mktemp -u "${TMPDIR:-/tmp}/lab-node.XXXX.iso")}
cleanup() { rm -rf "$WORK" "$WORK.log"; [ -n "$OUTPUT" ] || rm -f "$OUT"; }
trap cleanup EXIT

echo "Extracting $ISO ..."
xorriso -osirrox on -indev "$ISO" -extract / "$WORK" 2>/dev/null
chmod -R u+w "$WORK"

# Generic NoCloud seed (read by the installer at /cdrom/nocloud/). No network
# section -> Subiquity uses DHCP and waits for it before apt. No hostname/IP:
# those come from the DHCP reservation + the base role.
mkdir -p "$WORK/nocloud"
: > "$WORK/nocloud/meta-data"
cat > "$WORK/nocloud/user-data" <<EOF
#cloud-config
autoinstall:
  version: 1
  locale: en_GB.UTF-8
  keyboard:
    layout: gb
  identity:
    hostname: lab-node
    username: ${USERNAME}
    password: "!"
  ssh:
    install-server: true
    allow-pw: false
    authorized-keys:
      - "${KEY}"
  storage:
    layout:
      name: lvm
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
# Reuse the source ISO's boot layout verbatim (BIOS+UEFI). Flatten to one line so
# eval doesn't treat the per-option newlines as command separators.
FLAGS=$(xorriso -indev "$ISO" -report_el_torito as_mkisofs 2>/dev/null | tr '\n' ' ')
if ! eval xorriso -as mkisofs "$FLAGS" -o "$OUT" "$WORK" >"$WORK.log" 2>&1; then
  tail -20 "$WORK.log"; die "repack failed"
fi

if [ -n "$DEVICE" ]; then
  printf 'Write the generic node installer to %s (ERASES it)? [y/N] ' "$DEVICE"
  read -r ans; [ "$ans" = y ] || die "aborted"
  diskutil unmountDisk "$DEVICE" >/dev/null || true
  sudo dd if="$OUT" of="$DEVICE" bs=4m
  diskutil eject "$DEVICE" >/dev/null || true
  echo "Done. Boot any x86 node off it; the DHCP reservation lands it on its IP."
else
  echo "Wrote ${OUT} (generic node installer)."
fi
