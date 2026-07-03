# redis_sentinel

Installs Redis (master/replica) and Redis Sentinel for the Authentik HA data tier, deriving each host's role from its address rather than static assignment.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu target with systemd.
- Inventory group membership decides what installs: hosts in `authentik_redis` get `redis-server`; every targeted host gets `redis-sentinel` (so a witness can be Sentinel-only).
- Sentinel quorum members reachable on 26379 and Redis nodes on 6379.
- Single-homed nodes so `ansible_default_ipv4.address` self-reports correctly; a multi-homed witness must set `redis_sentinel_announce_ip`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `redis_conf_dir` | `/etc/redis` | Config directory (also holds the bootstrap marker). |
| `redis_data_dir` | `/var/lib/redis` | Redis and Sentinel working directory. |
| `redis_master_name` | `authentik` | Sentinel monitored-master name. |
| `redis_sentinel_quorum` | `2` | Sentinel votes required to agree the master is down. |
| `redis_sentinel_down_after_ms` | `5000` | Milliseconds unreachable before Sentinel marks the master subjectively down. |
| `redis_sentinel_failover_timeout_ms` | `15000` | Sentinel failover timeout (milliseconds). |
| `redis_this_ip` | `{{ ansible_default_ipv4.address }}` | This host's Redis address; compared to `redis_master_ip` to decide master vs replica. |
| `redis_announce_ip` | `{{ redis_sentinel_announce_ip \| default(ansible_default_ipv4.address) }}` | Address Sentinel announces; the multi-homed witness overrides via `redis_sentinel_announce_ip`. |

Required from inventory (no default): `redis_master_ip` — the address of the initial master.

SOPS secret (no default): `redis_password` — used for `requirepass`, `masterauth`, and Sentinel `auth-pass`.

## Dependencies

None (no `meta/main.yml`). At runtime the Sentinel quorum spans the `authentik_redis` data nodes plus any Sentinel-only witness. The `haproxy_patroni` role's optional Redis frontend consumes the master this quorum promotes.

## What it does

1. Installs `redis-server` only on hosts in the `authentik_redis` group.
2. Installs `redis-sentinel` on every targeted host.
3. Stats `{{ redis_conf_dir }}/.lab-configured` to decide whether this host has already been bootstrapped.
4. First bootstrap only (marker absent): renders `redis.conf` on data nodes and `sentinel.conf` on all hosts (0640, `redis:redis`), notifying the matching service restart. `redis.conf` emits a `replicaof {{ redis_master_ip }} 6379` line only when `redis_this_ip` differs from `redis_master_ip`, so the master starts standalone and the others start as its replicas.
5. First bootstrap only: touches the `.lab-configured` marker.
6. Enables and starts `redis-server` (data nodes) and `redis-sentinel` (all hosts).

## Example

```yaml
- hosts: authentik_redis:redis_witness
  roles:
    - role: redis_sentinel
      vars:
        redis_master_ip: 10.0.30.35
        redis_password: "{{ vault_redis_password }}"
        # on the cross-VLAN witness (host_var), so it advertises reachably:
        # redis_sentinel_announce_ip: 10.0.20.11
```

## Notes

- Config is written once, then left alone. Both Redis and Sentinel rewrite their own config files at runtime (`replicaof`, discovered peers, promoted master) after a Sentinel-driven failover; re-asserting the templates would fight those rewrites and could revert a failover. The `.lab-configured` marker enforces first-boot-only rendering.
- After a failover the true master no longer matches `redis_master_ip`. That variable only seeds the initial topology and Sentinel's monitor line; it is not the source of truth once the cluster is live.
- Quorum needs a third vote to break ties. Running Sentinel on the witness (without `redis-server`) gives three Sentinels across two data nodes, so the quorum of 2 can still promote a replica when one data node is lost.
- The multi-homed witness must announce its VLAN-30 address via `redis_sentinel_announce_ip`, otherwise Sentinel would advertise the wrong interface and peers could not reach it.
