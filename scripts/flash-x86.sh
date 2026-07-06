#!/usr/bin/env bash
# flash-x86.sh — remaster the Ubuntu 24.04 live-server ISO into an autoinstall USB.
# Two profiles (--profile, default node):
#
#   node — the headless x86 Incus cluster nodes (ThinkCentres). Fully unattended: DHCP,
#     the ansible user + baked SSH key, fresh per-install host keys. Per-host identity
#     comes from a DHCP reservation (terraform/unifi) + the base role, so one USB does
#     every node. Needs --pubkey. (node-ryzen runs Proxmox from its own installer.)
#
#   sheepdip — the air-gapped scanning station (Dell XPS 15). storage + identity are
#     INTERACTIVE, so the LUKS-encrypt + passphrase and login user are set by hand on the
#     box (nothing sensitive baked onto the USB); no SSH server. The rest is automated.
#
# macOS only. Needs xorriso (`brew install xorriso`). --device is destructive.
#
# Usage:
#   flash-x86.sh --iso ~/Downloads/ubuntu-24.04.x-live-server-amd64.iso \
#                --device /dev/disk5             # or: --output out.iso
#     node (default): --pubkey ~/.ssh/id_ed25519.pub [--user ansible]
#     sheepdip:       --profile sheepdip
set -euo pipefail

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
usage() { sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-0}"; }

PROFILE=node PUBKEY='' ISO='' DEVICE='' OUTPUT='' USERNAME=ansible
while [ $# -gt 0 ]; do
  case "$1" in
    --profile) PROFILE=$2; shift 2 ;;
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
case "$PROFILE" in node|sheepdip) ;; *) die "unknown --profile: $PROFILE (node|sheepdip)" ;; esac
[ -n "$ISO" ] || die "missing --iso"
[ -f "$ISO" ] || die "iso not found: $ISO"
[ -n "$DEVICE" ] || [ -n "$OUTPUT" ] || die "need --device or --output"
[ -z "$DEVICE" ] || [ -z "$OUTPUT" ] || die "use --device OR --output, not both"
KEY=''
if [ "$PROFILE" = node ]; then
  [ -n "$PUBKEY" ] || die "the node profile needs --pubkey"
  [ -f "$PUBKEY" ] || die "pubkey not found: $PUBKEY"
  KEY=$(cat "$PUBKEY")
fi

WORK=$(mktemp -d) || die "mktemp failed"
OUT=${OUTPUT:-$(mktemp -u "${TMPDIR:-/tmp}/lab-node.XXXX.iso")}
cleanup() { rm -rf "$WORK" "$WORK.log"; [ -n "$OUTPUT" ] || rm -f "$OUT"; }
trap cleanup EXIT

echo "Extracting $ISO ..."
xorriso -osirrox on -indev "$ISO" -extract / "$WORK" 2>/dev/null
chmod -R u+w "$WORK"

# NoCloud seed (read by the installer at /cdrom/nocloud/). Both profiles share the offline
# apt fallback (the lab DNS doesn't propagate into curtin's target resolv.conf, so the
# in-target network mirror can't resolve; /cdrom is made readable by the dir-mode fix
# below). node is fully unattended (DHCP + baked ssh key); sheepdip leaves storage +
# identity interactive so the LUKS passphrase + login are set by hand, with no SSH server.
mkdir -p "$WORK/nocloud"
: > "$WORK/nocloud/meta-data"
if [ "$PROFILE" = sheepdip ]; then
cat > "$WORK/nocloud/user-data" <<'EOF'
#cloud-config
autoinstall:
  version: 1
  interactive-sections:
    - storage
    - identity
  early-commands:
    - "bash /cdrom/nocloud/wipe-lvm.sh || true"
  locale: en_GB.UTF-8
  keyboard:
    layout: gb
  # DHCP any ethernet (incl. a USB dongle) so the box comes up networked for the online
  # build — the XPS has no built-in NIC and WiFi userspace isn't installed until the
  # playbook runs. `optional` so boot doesn't wait when nothing is plugged in.
  network:
    version: 2
    ethernets:
      alleth:
        match:
          name: "en*"
        dhcp4: true
        optional: true
  apt:
    geoip: false
    fallback: offline-install
  ssh:
    install-server: false
  shutdown: reboot
EOF
else
cat > "$WORK/nocloud/user-data" <<EOF
#cloud-config
autoinstall:
  version: 1
  early-commands:
    - "bash /cdrom/nocloud/wipe-lvm.sh || true"
  locale: en_GB.UTF-8
  keyboard:
    layout: gb
  apt:
    geoip: false
    fallback: offline-install
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
  shutdown: reboot
EOF
fi

# Pre-storage wipe of leftover LVM/RAID so curtin can reinstall over old attempts.
cat > "$WORK/nocloud/wipe-lvm.sh" <<'WIPE'
#!/bin/bash
swapoff -a 2>/dev/null || true
vgchange -an 2>/dev/null || true
for vg in $(vgs --noheadings -o vg_name 2>/dev/null); do vgremove -f -y "$vg" 2>/dev/null || true; done
for pv in $(pvs --noheadings -o pv_name 2>/dev/null); do pvremove -ff -y "$pv" 2>/dev/null || true; done
mdadm --stop --scan 2>/dev/null || true
WIPE

# Boot the autoinstall entry immediately, unattended.
GRUB="$WORK/boot/grub/grub.cfg"
sed -i '' 's/^set timeout=.*/set timeout=1/' "$GRUB"
sed -i '' 's#/casper/vmlinuz  *---#/casper/vmlinuz autoinstall ds=nocloud\\;s=/cdrom/nocloud/ ---#' "$GRUB"

# grub.cfg is listed in the ISO's md5 manifest; refresh its hash or casper's
# media-integrity check fails the install on our edit.
NEW_MD5=$(md5 -q "$GRUB")
sed -i '' "s#^[0-9a-f]*  ./boot/grub/grub.cfg\$#${NEW_MD5}  ./boot/grub/grub.cfg#" "$WORK/md5sum.txt"

echo "Repacking ..."
# Reuse the source ISO's boot layout verbatim (BIOS+UEFI). Flatten to one line so
# eval doesn't treat the per-option newlines as command separators. -dir-mode 0755
# is critical: mktemp makes the work dir 0700, which the repack bakes onto the ISO
# root, leaving /cdrom unreadable by the _apt user so the in-target offline apt pool
# fails and the install falls back to the network (LP #1963725).
FLAGS=$(xorriso -indev "$ISO" -report_el_torito as_mkisofs 2>/dev/null | tr '\n' ' ')
if ! eval xorriso -as mkisofs "$FLAGS" -dir-mode 0755 -o "$OUT" "$WORK" >"$WORK.log" 2>&1; then
  tail -20 "$WORK.log"; die "repack failed"
fi

if [ -n "$DEVICE" ]; then
  printf 'Write the %s installer to %s (ERASES it)? [y/N] ' "$PROFILE" "$DEVICE"
  read -r ans; [ "$ans" = y ] || die "aborted"
  diskutil unmountDisk "$DEVICE" >/dev/null || true
  sudo dd if="$OUT" of="$DEVICE" bs=4m
  diskutil eject "$DEVICE" >/dev/null || true
  if [ "$PROFILE" = sheepdip ]; then
    echo "Done. Boot the XPS off it; drive the encryption + user screens, then run the sheepdip playbook."
  else
    echo "Done. Boot any x86 node off it; the DHCP reservation lands it on its IP."
  fi
else
  echo "Wrote ${OUT} (${PROFILE} installer)."
fi
