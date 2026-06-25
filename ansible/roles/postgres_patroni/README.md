# postgres_patroni

PostgreSQL 16 (PGDG) under Patroni, with automatic failover via the etcd quorum.
Patroni owns Postgres — the distro `postgresql` service is masked and Patroni
manages initdb, replication, leader election, and promotion.

## How it works

- Installs PG16 + Patroni (`patroni[etcd3]`) in a venv at `/opt/patroni`.
- Removes the apt-created default cluster **once** (guarded by a marker so a re-run
  can never drop the cluster Patroni has bootstrapped) so Patroni initdbs its own.
- Renders `/etc/patroni/patroni.yml` and runs `patroni.service`.
- After a leader is elected (polls `/cluster`), creates the `authentik` role + db
  on the leader, idempotently and `run_once`.

## Required inventory vars

`patroni_etcd_hosts` (e.g. `10.200.30.31:2379,10.200.30.32:2379,10.200.30.13:2379`
— the Pi entry is its VLAN-30 foot), and the SOPS secrets
`patroni_superuser_password`, `patroni_replication_password`,
`patroni_rest_password`, `patroni_app_password`.

## App-tier contract

App nodes reach the current primary through the local `haproxy_patroni`
(`127.0.0.1:5000`), database `authentik`, user `authentik` /
`patroni_app_password`.

## Operations

`patronictl -c /etc/patroni/patroni.yml list` shows roles/lag.
`ttl ≥ loop_wait + 2·retry_timeout` (30 ≥ 10 + 20) — keep that invariant if you
retune the timers. A watchdog (softdog) is a sensible later hardening for extra
split-brain protection.
