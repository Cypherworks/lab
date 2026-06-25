# redis_sentinel

Redis with Sentinel-driven automatic failover for the Authentik HA data tier.

- **Redis server** installs on hosts in `authentik_redis` (master + replica). The
  master/replica role is derived: a host whose address ≠ `redis_master_ip` gets
  `replicaof`.
- **Sentinel** installs on every targeted host, including the Pi witness — which
  runs **Sentinel only** (it's not in `authentik_redis`, so no `redis-server`).
- The Pi announces on its VLAN-30 foot via `redis_sentinel_announce_ip`
  (host_var = `10.200.30.13`); single-homed containers announce their own address.

## Required inventory vars

`redis_master_ip`, the SOPS secret `redis_password`, and on the Pi witness:
`redis_sentinel_announce_ip: 10.200.30.13`.

## ⚠️ Configs are written once (`force: false`)

Redis and Sentinel **rewrite their own configs** at runtime (`replicaof` after a
failover; Sentinel's discovered topology). So the templates create the files only
if absent — re-runs do **not** overwrite them, or we'd reset the cluster's learned
state. To change static settings (timings, password), update the template and
remove the file (or edit in place) on a deliberate, controlled pass.
