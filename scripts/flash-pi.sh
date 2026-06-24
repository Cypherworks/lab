#!/usr/bin/env bash
# flash-pi.sh — write an Ubuntu Server (Pi, arm64) image to an SD/USB and inject
# headless cloud-init so the Pi boots straight onto the lab network with SSH and
# a static IP. No screen, no keyboard.
#
# macOS only (the operator's MacBook is the imaging host). Destructive: it wipes
# the target device. The per-host data (IP/gateway/key) is passed in by the
# caller — this script is mechanism; site data lives in homelab-deploy.
#
# Usage:
#   flash-pi.sh --hostname pi-dns-1 --ip 10.200.20.11 --gateway 10.200.20.1 \
#               --device /dev/disk4 --pubkey ~/.ssh/id_ed25519.pub \
#               --image ubuntu-24.04-preinstalled-server-arm64+raspi.img.xz \
#               [--prefix 24] [--interface eth0] [--dns 1.1.1.1] [--user ansible]
set -euo pipefail

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
usage() { sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-0}"; }

HOSTNAME='' IP='' GATEWAY='' DEVICE='' PUBKEY='' IMAGE=''
PREFIX=24 IFACE=eth0 DNS=1.1.1.1 USERNAME=ansible
while [ $# -gt 0 ]; do
  case "$1" in
    --hostname) HOSTNAME=$2; shift 2 ;;
    --ip) IP=$2; shift 2 ;;
    --gateway) GATEWAY=$2; shift 2 ;;
    --device) DEVICE=$2; shift 2 ;;
    --pubkey) PUBKEY=$2; shift 2 ;;
    --image) IMAGE=$2; shift 2 ;;
    --prefix) PREFIX=$2; shift 2 ;;
    --interface) IFACE=$2; shift 2 ;;
    --dns) DNS=$2; shift 2 ;;
    --user) USERNAME=$2; shift 2 ;;
    -h|--help) usage 0 ;;
    *) die "unknown arg: $1" ;;
  esac
done

[ "$(uname)" = "Darwin" ] || die "this script targets macOS"
for v in HOSTNAME IP GATEWAY DEVICE PUBKEY IMAGE; do
  [ -n "${!v}" ] || die "--${v,,} is required (see --help)"
done
[ -f "$PUBKEY" ] || die "pubkey not found: $PUBKEY"
[ -f "$IMAGE" ]  || die "image not found: $IMAGE"
[ -b "$DEVICE" ] || die "not a block device: $DEVICE (try: diskutil list)"

echo "Target device:"
diskutil info "$DEVICE" | grep -E 'Device / Media Name|Disk Size|Device Node' || true
echo
echo "This will ERASE $DEVICE and flash $HOSTNAME ($IP/$PREFIX gw $GATEWAY)."
read -r -p "Type the device node again to confirm (e.g. $DEVICE): " confirm
[ "$confirm" = "$DEVICE" ] || die "confirmation did not match; aborting"

RAW="${DEVICE/disk/rdisk}"   # raw node is much faster for dd on macOS
echo "==> Unmounting $DEVICE"
diskutil unmountDisk "$DEVICE"

echo "==> Writing image (this takes a few minutes)"
case "$IMAGE" in
  *.xz) xzcat "$IMAGE" | sudo dd of="$RAW" bs=4m status=progress ;;
  *.img) sudo dd if="$IMAGE" of="$RAW" bs=4m status=progress ;;
  *) die "image must be .img or .img.xz" ;;
esac
sync

echo "==> Mounting the boot partition"
diskutil mountDisk "$DEVICE" >/dev/null
BOOT=/Volumes/system-boot
for _ in $(seq 1 10); do [ -d "$BOOT" ] && break; sleep 1; done
[ -d "$BOOT" ] || die "boot partition (system-boot) did not mount"

PUBKEY_CONTENT=$(cat "$PUBKEY")
echo "==> Injecting cloud-init"
cat > "$BOOT/meta-data" <<EOF
instance-id: ${HOSTNAME}
local-hostname: ${HOSTNAME}
EOF

cat > "$BOOT/network-config" <<EOF
version: 2
ethernets:
  ${IFACE}:
    dhcp4: false
    dhcp6: false
    addresses: [${IP}/${PREFIX}]
    routes:
      - to: default
        via: ${GATEWAY}
    nameservers:
      addresses: [${DNS}]
EOF

cat > "$BOOT/user-data" <<EOF
#cloud-config
hostname: ${HOSTNAME}
manage_etc_hosts: true
users:
  - name: ${USERNAME}
    groups: [sudo]
    shell: /bin/bash
    sudo: "ALL=(ALL) NOPASSWD:ALL"
    lock_passwd: true
    ssh_authorized_keys:
      - ${PUBKEY_CONTENT}
ssh_pwauth: false
package_update: true
packages:
  - openssh-server
  - python3
runcmd:
  - [systemctl, enable, --now, ssh]
EOF

sync
echo "==> Ejecting"
diskutil eject "$DEVICE"
echo "Done. Insert into ${HOSTNAME}, power on, then run the provision poller."
