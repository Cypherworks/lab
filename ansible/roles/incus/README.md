# incus

Installs and initialises a standalone Incus host: a btrfs storage pool on a
dedicated LV carved from the OS volume group, a remote HTTPS API listener, and a
bridged NIC profile per lab VLAN so instances sit directly on the network rather
than behind NAT.

Standalone, not clustered — the x86 nodes run independent Incus daemons (the burst
node is offline for long stretches, which makes it a poor cluster member). Manage
the hosts as separate Incus remotes (Terraform/CLI).

## What it does

- Installs `incus` (daemon + CLI, with VM support) plus the LVM and btrfs tooling.
- Carves a dedicated LV from the OS volume group and lets Incus format it btrfs.
- Adds the `ansible` user to `incus-admin` for non-root management.
- Builds `<uplink>.<vlan>` VLAN interfaces and bridges in netplan over the trunked
  uplink; the bridges carry no host IP.
- Initialises Incus once from a preseed (storage pool + listener + default root
  disk), guarded so re-runs don't re-apply the non-idempotent preseed.
- Creates an Incus profile per VLAN with a bridged NIC on that VLAN's bridge.

Launch an instance onto a VLAN by stacking profiles, e.g.
`incus launch images:debian/13 web -p default -p services`.

## Variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `incus_packages` | incus, lvm2, thin-provisioning-tools | Packages to install |
| `incus_storage_pool` | `default` | Storage pool name |
| `incus_storage_vg` | `ubuntu-vg` | VG to carve the storage LV from |
| `incus_storage_lv` | `incus` | LV name for the btrfs pool |
| `incus_storage_size` | `120g` | Size of the storage LV |
| `incus_https_address` | `[::]:8443` | Remote API listen address |
| `incus_uplink_interface` | `lab0` | Trunked host NIC carrying tagged VLANs |
| `incus_networks` | services / 30 / br30 | VLANs exposed to instances |

Host-specific data (a different VG, extra VLANs) goes in host/group vars, not here.

## Requirements

The host's switch port must be a trunk carrying the tagged VLANs in
`incus_networks` (the `unifi-switch-ports` trunk profile already does this for the
cluster nodes).
