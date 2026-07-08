# etcd

Installs a pinned etcd cluster to serve as the distributed configuration store (DCS) that Patroni uses for leader election and automatic PostgreSQL failover.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu target with systemd.
- Outbound access to GitHub releases to fetch the pinned etcd tarball (amd64 or arm64, selected from `ansible_architecture`).
- Every member listed in `etcd_members` must be reachable on TCP 2379 (client) and 2380 (peer).
- `inventory_hostname` of each target must match a `name` entry in `etcd_members` so the role can derive that host's bind IP.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `etcd_version` | `3.5.16` | Pinned etcd release to download. |
| `etcd_arch` | `arm64` if `ansible_architecture == aarch64` else `amd64` | Release architecture, derived per host. |
| `etcd_release_base` | `https://github.com/etcd-io/etcd/releases/download` | Base URL for release downloads. |
| `etcd_archive` | `etcd-v{{ etcd_version }}-linux-{{ etcd_arch }}.tar.gz` | Tarball filename. |
| `etcd_url` | `{{ etcd_release_base }}/v{{ etcd_version }}/{{ etcd_archive }}` | Full download URL. |
| `etcd_data_dir` | `/var/lib/etcd` | etcd data directory. |
| `etcd_conf_dir` | `/etc/etcd` | Config directory. |
| `etcd_config_file` | `{{ etcd_conf_dir }}/etcd.conf.yml` | Rendered config path. |
| `etcd_cluster_token` | `authentik-etcd` | `initial-cluster-token` shared by all members. |
| `etcd_initial_cluster_state` | `new` | `new` on first bring-up; set to `existing` (per host, temporarily) when re-adding a member to a live cluster. |
| `etcd_bind_ip` | derived from `etcd_members` by matching `inventory_hostname` | This host's bind address for peer and client URLs. |

Required from inventory (no default): `etcd_members` — the full member list, each entry `{ name: <inventory_hostname>, ip: <bind IP> }`.

## Dependencies

None (no `meta/main.yml`). At runtime this role is the DCS that `postgres_patroni` depends on; stand up the etcd cluster before Patroni starts.

## What it does

1. Creates the `etcd` system group and user (nologin, home at the data dir, no home created).
2. Downloads the pinned release tarball to `/tmp` and extracts only the `etcd` and `etcdctl` binaries into `/usr/local/bin` (guarded by `creates`).
3. Sets the binaries executable and root-owned.
4. Creates the config directory (`root:etcd`, 0750) and data directory (`etcd:etcd`, 0700).
5. For a **fresh** node, probes the other members: if a live cluster answers it registers this node with `etcdctl member add` and starts it as an `existing` member; if none answer it bootstraps a `new` cluster. An already-initialised member skips this (its `initial-cluster*` are inert), so adding a node never bounces the running members.
6. Renders `etcd.conf.yml` from `etcd_members` (fresh/joining nodes only), building `initial-cluster` from every member. The host binds its `etcd_members` IP (so a multi-homed witness binds its VLAN-30 foot, not another interface), plus `127.0.0.1` for local clients.
7. Installs the systemd unit and enables/starts etcd, reloading systemd and restarting on config or unit change.

## Example

```yaml
- hosts: etcd
  roles:
    - role: etcd
      vars:
        etcd_members:
          - { name: etcd-1, ip: 10.0.30.31 }
          - { name: etcd-2, ip: 10.0.30.32 }
          - { name: witness, ip: 10.0.20.11 }   # 3rd member may sit on another VLAN
```

## Notes

- Adding a member is automatic: put it in `etcd_members` and converge — a fresh node joins a live cluster on its own (`member add` + `existing`), no manual per-host state flip. `etcd_initial_cluster_state` (`new`) is only the fallback for initial formation when no peer answers.
- **Removing** a member is deliberate and manual: `etcdctl member remove <id>` against a healthy member, then drop it from `etcd_members`. The role never removes members, so a converge can't break quorum.
- Quorum needs an odd count. The three-member layout (two data nodes plus the Pi witness) gives a third vote so the cluster survives losing one node and Patroni can still elect a Postgres leader.
- The client URL includes `127.0.0.1:2379` so local `etcdctl` works without hitting the network bind address.
