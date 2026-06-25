# incus

Installs and initialises a clustered Incus host: a btrfs storage pool on a
dedicated LV carved from the OS volume group, a remote HTTPS API listener, and a
bridged NIC profile per lab VLAN so instances sit directly on the network rather
than behind NAT.

Clustered for a single management plane (one web UI / API across all members),
with local storage per member and no shared storage. The bootstrap member converts
in place; the rest join with a single-use token minted on it. Quorum sits on the
always-on members, so a burst node that's offline for long stretches doesn't break
the cluster — its instances are simply unavailable while it's down.

## What it does

- Installs `incus` (daemon + CLI, with VM support) plus the LVM and btrfs tooling.
- Carves a dedicated LV from the OS volume group and lets Incus format it btrfs.
- Adds the `ansible` user to `incus-admin` for non-root management.
- Builds `<uplink>.<vlan>` VLAN interfaces and bridges in netplan over the trunked
  uplink; the bridges carry no host IP.
- Bootstraps the cluster on the first member (preseed for storage + listener +
  default root disk, then `incus cluster enable`); other members join with a
  single-use token minted on the bootstrap. All steps guarded for idempotency.
- Creates an Incus profile per VLAN (on the bootstrap; profiles are cluster-wide)
  with a bridged NIC on that VLAN's bridge.

Launch an instance onto a VLAN by stacking profiles, e.g.
`incus launch images:debian/13 web -p default -p services`.

## Variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `incus_packages` | incus, lvm2, btrfs-progs | Packages to install |
| `incus_storage_pool` | `default` | Storage pool name |
| `incus_storage_vg` | `ubuntu-vg` | VG to carve the storage LV from |
| `incus_storage_lv` | `incus` | LV name for the btrfs pool |
| `incus_storage_size` | `120g` | Size of the storage LV |
| `incus_https_address` | `[::]:8443` | Remote API listen address |
| `incus_uplink_interface` | default-route NIC | Trunked host NIC carrying tagged VLANs |
| `incus_networks` | services / 30 / br30 | VLANs exposed to instances |
| `incus_cluster_enabled` | `true` | Form/join a cluster |
| `incus_cluster_bootstrap_host` | `""` | inventory_hostname of the bootstrap member (required when clustering) |

Host-specific data (a different VG, extra VLANs) goes in host/group vars, not here.

## Requirements

The host's switch port must be a trunk carrying the tagged VLANs in
`incus_networks` (the `unifi-switch-ports` trunk profile already does this for the
cluster nodes).
