# incus

Installs Incus LTS 7.0 with dedicated btrfs storage, per-VLAN instance bridges, and optional clustering. Management is via the CLI and remote API (no web UI).

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
| `incus_zabbly_channel` | `lts-7.0` | Zabbly repo channel; gives Incus 7.0.x LTS. |
| `incus_packages` | `[incus]` | Packages installed from Zabbly (the Incus daemon). |
| `incus_hold_packages` | `[incus, incus-base, incus-client]` | Packages pinned to the Zabbly origin and held, so neither apt nor unattended-upgrades can downgrade or autoremove them. |
| `incus_tooling` | `[lvm2, btrfs-progs]` | Storage tooling installed from the OS archive. |
| `incus_storage_pool` | `default` | Incus storage pool name. |
| `incus_storage_vg` | `ubuntu-vg` | Volume group the storage LV is carved from. |
| `incus_storage_lv` | `incus` | Dedicated LV Incus formats btrfs. |
| `incus_storage_size` | `120g` | Size of the storage LV; override per host. |
| `incus_scrub_enabled` | `true` | Install the weekly btrfs scrub service + timer for the storage pool. |
| `incus_scrub_schedule` | `Sun *-*-* 03:00:00` | systemd `OnCalendar` schedule for the scrub. |
| `incus_scrub_pool_path` | `/var/lib/incus/storage-pools/{{ incus_storage_pool }}` | Btrfs path scrubbed. |
| `incus_https_address` | `[::]:8443` | Remote API listener for the Incus client and Terraform. |
| `incus_metrics_port` | `8444` | Port for the Prometheus `/1.0/metrics` listener. |
| `incus_metrics_cert_pem` | `""` | Metrics scraper client cert (PEM); empty skips the metrics listener and trust. |
| `incus_uplink_interface` | `{{ lab_interface \| default(ansible_default_ipv4.interface) }}` | Trunk NIC carrying the tagged VLANs; override per host with the real kernel name. |
| `incus_networks` | `[{name: services, vlan: 30, bridge: br30}, {name: workbench, vlan: 55, bridge: br55}]` | Lab VLANs exposed to instances; each renders a netplan VLAN link and an Incus profile with a bridged NIC. |
| `incus_cluster_enabled` | `true` | Whether to form/join a cluster. |
| `incus_cluster_bootstrap_host` | `""` | `inventory_hostname` of the bootstrap member; required when clustering. |
| `incus_cluster_member_name` | `{{ inventory_hostname }}` | This member's cluster name. |
| `incus_cluster_address` | `{{ ansible_default_ipv4.address }}` | Address this member advertises to the cluster (bound before enabling; wildcards are rejected). |
| `incus_placement_scriptlet_enabled` | `true` | Load the anti-affinity placement scriptlet (cluster-global; set on the bootstrap member). |
| `incus_boot_gate_enabled` | `true` | Install the cold-boot gate that holds the Incus daemon until the lab gateway answers. |
| `incus_boot_gate_target` | `{{ ansible_default_ipv4.gateway }}` | Address the gate pings before releasing the daemon. |
| `incus_boot_gate_timeout` | `300` | Seconds after which the gate releases the daemon anyway (fail-open). |

## Dependencies

None (no `meta/main.yml`). Requires the `community.general` and `community.docker`-independent collections noted above; storage tooling is installed by the role.

## What it does

1. Adds the Zabbly apt repository and key, pins Incus to the Zabbly origin (so apt and unattended-upgrades can never resolve it to the Ubuntu archive's incus), installs the storage tooling, then installs Incus, removes the `incus-ui-canonical` web UI if present, and holds `incus_hold_packages` against unattended-upgrades.
2. Adds `ansible_user` to `incus-admin` so it can drive Incus without root.
3. Renders `/etc/netplan/70-incus.yaml` with a `<uplink>.<vlan>` link and bridge per entry in `incus_networks`, then flushes handlers so the bridges are up before any profile references them.
4. Carves the dedicated storage LV.
5. On the bootstrap member (or a standalone host), initialises Incus from preseed; when clustering, binds the API to the member's real IP and runs `incus cluster enable`.
6. Creates a profile per lab VLAN with a bridged NIC on that VLAN's bridge.
7. On a joining member, mints a single-use token on the bootstrap host (via `delegate_to`) and joins with `incus admin init --preseed`.
8. On the bootstrap member, loads the anti-affinity placement scriptlet (`instances.placement.scriptlet`).
9. Installs the weekly btrfs scrub service + timer, which reads and verifies every block in the storage pool against its checksum (`incus_scrub_enabled`).
10. Installs the cold-boot gate (script, service, and Incus service drop-in) that holds the daemon at boot until the lab gateway answers, so it doesn't form the cluster or autostart instances into a dead network (`incus_boot_gate_enabled`).
11. When `incus_metrics_cert_pem` is set, binds the Prometheus metrics listener on each member and trusts the scraper's client cert cluster-wide (metrics-only).

The host keeps its own address on the uplink; the per-VLAN bridges carry no host IP. Storage stays local per member — the cluster needs no shared storage.

## Instance placement

`files/instance-placement.star` keeps same-group instances on separate hosts. The group is the instance name without a trailing `-<n>`, so `etcd-1`/`etcd-2`/`etcd-3` are one group and never share a node; among the conflict-free hosts the emptiest (most free memory) wins. It replaces static `target` pinning — instances carry no node pin and the scriptlet places them on create and on evacuation/rebalance. It fails safe: if no conflict-free host exists or resources can't be read, it returns without a target and Incus uses its built-in placement, so a scriptlet fault never blocks instance creation. `cluster.rebalance.*` is left disabled — it only live-migrates VMs, and this is an all-container fleet.

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

- The Zabbly pin keeps apt off the Ubuntu archive's incus (a 6.0.0-vs-7.0 conflict that once got the whole stack autoremoved). Upgrade cluster members together to avoid prolonged version skew.
- The `Apply incus netplan` handler is named distinctly from the base role's `Apply netplan` so the two don't collide in a combined play.
- Incus's LVM driver insists on owning a whole empty VG, so the role gives it a dedicated block device (the btrfs-formatted LV) rather than sharing the VG.
- The cluster address must be a specific IP — clustering rejects the wildcard listener, so the bootstrap API is rebound before `cluster enable`.
