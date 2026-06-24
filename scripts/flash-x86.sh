#!/usr/bin/env bash
# flash-x86.sh — write a NoCloud autoinstall "seed" USB for an x86 lab host
# (ThinkCentre / Ryzen). Ubuntu 24.04 live-server reads it and installs fully
# unattended onto the host's planned static IP, with the ansible user + SSH key.
#
# Two USB sticks are used:
#   1. a GENERIC Ubuntu 24.04 live-server installer USB (made once with
#      `dd`/balenaEtcher from the .iso — not this script's job, it's reusable);
#   2. this small "seed" USB (per host), written here.
# Boot the host from the installer USB with the seed USB also plugged in. At the
# GRUB menu press `e`, append ` autoinstall` to the `linux` line, then boot
# (Ctrl-X / F10) so the install runs without the confirmation prompt.
#
# macOS only. Destructive: it wipes the target seed device. Per-host data is
# passed in by the caller — this is mechanism; site data lives in homelab-deploy.
#
# Usage:
#   flash-x86.sh --hostname node-tc1 --ip 10.200.20.21 --gateway 10.200.20.1 \
#                --device /dev/disk5 --pubkey ~/.ssh/id_ed25519.pub \
#                [--prefix 24] [--dns 1.1.1.1] [--user ansible] [--match 'en*']
set -euo pipefail

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
usage() { sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-0}"; }

HOSTNAME='' IP='' GATEWAY='' DEVICE='' PUBKEY=''
PREFIX=24 DNS=1.1.1.1 USERNAME=ansible MATCH='en*'
while [ $# -gt 0 ]; do
  case "$1" in
    --hostname) HOSTNAME=$2; shift 2 ;;
    --ip) IP=$2; shift 2 ;;
    --gateway) GATEWAY=$2; shift 2 ;;
    --device) DEVICE=$2; shift 2 ;;
    --pubkey) PUBKEY=$2; shift 2 ;;
    --prefix) PREFIX=$2; shift 2 ;;
    --dns) DNS=$2; shift 2 ;;
    --user) USERNAME=$2; shift 2 ;;
    --match) MATCH=$2; shift 2 ;;
    -h|--help) usage 0 ;;
    *) die "unknown arg: $1" ;;
  esac
done

[ "$(uname)" = "Darwin" ] || die "this script targets macOS"
for v in HOSTNAME IP GATEWAY DEVICE PUBKEY; do
  [ -n "${!v}" ] || die "missing --${v,,}"
done
[ -f "$PUBKEY" ] || die "pubkey not found: $PUBKEY"
[ -e "$DEVICE" ] || die "device not found: $DEVICE"
KEY=$(cat "$PUBKEY")

printf 'About to ERASE %s and write the seed for %s (%s). Continue? [y/N] ' "$DEVICE" "$HOSTNAME" "$IP"
read -r ans; [ "$ans" = y ] || die "aborted"

# FAT32, volume label CIDATA — cloud-init's NoCloud datasource looks for it.
diskutil eraseDisk FAT32 CIDATA MBRFormat "$DEVICE"
MNT=/Volumes/CIDATA
[ -d "$MNT" ] || die "expected $MNT after format"

cat > "$MNT/meta-data" <<EOF
instance-id: ${HOSTNAME}
local-hostname: ${HOSTNAME}
EOF

# Subiquity autoinstall. The ansible account is key-only (locked password) with
# NOPASSWD sudo, matching the Pi build. Static IP on the first wired NIC (matched
# by glob, since x86 NIC names vary); base/CIS take over once it's on the network.
cat > "$MNT/user-data" <<EOF
#cloud-config
autoinstall:
  version: 1
  locale: en_GB.UTF-8
  keyboard:
    layout: gb
  identity:
    hostname: ${HOSTNAME}
    username: ${USERNAME}
    # Locked password — login is key-only. (Subiquity requires the field.)
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

sync
diskutil eject "$DEVICE" >/dev/null
cat <<EOF

Seed written for ${HOSTNAME} (${IP}).
Next:
  1. Plug this seed USB + the generic Ubuntu 24.04 installer USB into ${HOSTNAME}.
  2. Boot the installer USB; at GRUB press 'e', append ' autoinstall' to the
     'linux' line, then Ctrl-X to boot. It installs unattended and reboots.
  3. The host comes up at ${IP}; provision with: ansible-playbook playbooks/node.yml -l ${HOSTNAME}
EOF
