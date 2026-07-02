# haproxy_patroni

Installs a local HAProxy on each Authentik app node that routes PostgreSQL traffic to the current Patroni primary, with an optional frontend that follows the Sentinel-promoted Redis master.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu target with systemd.
- Patroni members reachable on 5432 (Postgres) and their REST port (default 8008) for the `/primary` healthcheck.
- For the optional Redis frontend: Redis backends reachable on 6379 and the matching AUTH password.
- Single-homed nodes so `ansible_default_ipv4.address` is the correct lab address, or override the bind variables.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `haproxy_patroni_bind` | `{{ ansible_default_ipv4.address }}:5000` | Address/port the Postgres-primary frontend binds. |
| `haproxy_patroni_rest_port` | `8008` | Patroni REST port used for the `/primary` healthcheck. |
| `haproxy_redis_bind` | `{{ ansible_default_ipv4.address }}:6379` | Address/port the optional Redis-master frontend binds. |
| `haproxy_redis_backends` | `[]` | Redis backend list; non-empty enables the Redis frontend. Each entry `{ name, ip }`. |
| `haproxy_redis_password` | `""` | Redis AUTH password for the master healthcheck. From SOPS. |

Required from inventory (no default): `patroni_members` — the Patroni backends, each entry `{ name, ip }`.

SOPS secret: `haproxy_redis_password` (only when the Redis frontend is enabled).

## Dependencies

None (no `meta/main.yml`). At runtime it depends on a running Patroni cluster (`postgres_patroni`) for the primary healthcheck, and optionally on `redis_sentinel` backends for the Redis frontend.

## What it does

1. Installs the `haproxy` package.
2. Renders `/etc/haproxy/haproxy.cfg` (mode 0640, because it may carry the Redis AUTH password), validated with `haproxy -c -f` before it is accepted, notifying an HAProxy reload on change.
3. Enables and starts HAProxy.

The rendered config defines a `postgres-primary` TCP frontend bound to `haproxy_patroni_bind`, with each `patroni_members` entry as a backend server health-checked via `httpchk OPTIONS /primary` on `haproxy_patroni_rest_port`. Patroni's REST returns 200 for `/primary` only on the leader, so exactly one backend is up at a time and traffic follows failover.

When `haproxy_redis_backends` is non-empty, it also defines a `redis-master` TCP frontend bound to `haproxy_redis_bind` that health-checks each backend with an AUTH/PING/`info replication` sequence and accepts only the node reporting `role:master`, following the Sentinel-promoted master.

## Example

```yaml
- hosts: authentik_app
  roles:
    - role: haproxy_patroni
      vars:
        patroni_members:
          - { name: pg-tc1, ip: 10.200.30.33 }
          - { name: pg-tc2, ip: 10.200.30.34 }
        haproxy_redis_backends:
          - { name: redis-tc1, ip: 10.200.30.35 }
          - { name: redis-tc2, ip: 10.200.30.36 }
        haproxy_redis_password: "{{ vault_redis_password }}"
```

## Notes

- The frontends bind the node's lab IP, not loopback, because the Authentik containers run on the compose bridge (not host networking) and reach HAProxy through the node's address.
- Routing is by healthcheck, not static config: the `/primary` check means HAProxy always points at the current leader without re-rendering after a failover.
- Authentik has no Sentinel client, so it targets the single `haproxy_redis_bind` endpoint; Sentinel performs the failover and the `role:master` check makes HAProxy follow the new master.
- The config is validated (`validate: haproxy -c -f %s`) before replacing the live file, so a bad render fails the task instead of taking HAProxy down.
