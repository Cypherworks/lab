# incus

Installs Incus LTS 7.0 with the web UI, dedicated btrfs storage, per-VLAN instance bridges, and optional clustering.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Ubuntu (or a Debian-family release the Zabbly repo builds for); the role uses `deb822_repository`, so a recent apt.
- A volume group with free space for the storage LV (`incus_storage_vg`, default `ubuntu-vg`).
- `community.general` (for `lvol`) and `netplan`-managed networking on the host.
- The uplink NIC must be a trunk port carrying the tagged lab VLANs.
- For clustering: `incus_cluster_bootstrap_host` set to the bootstrap member's `inventory_hostname`, and the bootstrap host reachable from the joining members over the API.

## Role variables

| Variable | Default | Description |
| --- | --- | --- |
| `incus_zabbly_channel` | `lts-7.0` | Zabbly repo channel; gives Incus 7.0.x LTS plus `incus-ui-canonical`. |
| `incus_packages` | `[incus, incus-ui-canonical]` | Packages installed from Zabbly (daemon + daemon-served web UI). |
| `incus_tooling` | `[lvm2, btrfs-progs]` | Storage tooling installed from the OS archive. |
| `incus_storage_pool` | `default` | Incus storage pool name. |
| `incus_storage_vg` | `ubuntu-vg` | Volume group the storage LV is carved from. |
| `incus_storage_lv` | `incus` | Dedicated LV Incus formats btrfs. |
| `incus_storage_size` | `120g` | Size of the storage LV; override per host. |
| `incus_https_address` | `[::]:8443` | Remote API listener for the Incus client and Terraform. |
| `incus_uplink_interface` | `{{ ansible_default_ipv4.interface }}` | Trunk NIC carrying the tagged VLANs; override per host with the real kernel name. |
| `incus_networks` | `[{name: services, vlan: 30, bridge: br30}]` | Lab VLANs exposed to instances; each renders a netplan VLAN link and an Incus profile with a bridged NIC. |
| `incus_cluster_enabled` | `true` | Whether to form/join a cluster. |
| `incus_cluster_bootstrap_host` | `""` | `inventory_hostname` of the bootstrap member; required when clustering. |
| `incus_cluster_member_name` | `{{ inventory_hostname }}` | This member's cluster name. |
| `incus_cluster_address` | `{{ ansible_default_ipv4.address }}` | Address this member advertises to the cluster (bound before enabling; wildcards are rejected). |

## Dependencies

None (no `meta/main.yml`). Requires the `community.general` and `community.docker`-independent collections noted above; storage tooling is installed by the role.

## What it does

1. Adds the Zabbly apt repository and key, installs the storage tooling, then installs Incus and the web UI.
2. Adds `ansible_user` to `incus-admin` so it can drive Incus without root.
3. Renders `/etc/netplan/70-incus.yaml` with a `<uplink>.<vlan>` link and bridge per entry in `incus_networks`, then flushes handlers so the bridges are up before any profile references them.
4. Carves the dedicated storage LV.
5. On the bootstrap member (or a standalone host), initialises Incus from preseed; when clustering, binds the API to the member's real IP and runs `incus cluster enable`.
6. Creates a profile per lab VLAN with a bridged NIC on that VLAN's bridge.
7. On a joining member, mints a single-use token on the bootstrap host (via `delegate_to`) and joins with `incus admin init --preseed`.

The host keeps its own address on the uplink; the per-VLAN bridges carry no host IP. Storage stays local per member — the cluster needs no shared storage.

## Example

```yaml
- hosts: incus_nodes
  become: true
  roles:
    - role: incus
      vars:
        incus_cluster_bootstrap_host: tc1
        incus_uplink_interface: enp0s31f6
        incus_storage_size: 200g
        incus_networks:
          - name: services
            vlan: 30
            bridge: br30
          - name: dmz
            vlan: 40
            bridge: br40
```

## Notes

- The web UI depends on the matching Incus version, so installing it pulls Incus up to the Zabbly LTS. Upgrade cluster members together to avoid prolonged version skew.
- The `Apply incus netplan` handler is named distinctly from the base role's `Apply netplan` so the two don't collide in a combined play.
- Incus's LVM driver insists on owning a whole empty VG, so the role gives it a dedicated block device (the btrfs-formatted LV) rather than sharing the VG.
- The cluster address must be a specific IP — clustering rejects the wildcard listener, so the bootstrap API is rebound before `cluster enable`.
