# postgres_patroni

Installs PostgreSQL 16 (PGDG) managed by Patroni, with automatic leader election and failover through an etcd DCS.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu target with systemd; a minimal cloud image is fine (the role installs the Python prerequisites it needs).
- The `community.postgresql` collection on the controller (used to create the application role and database).
- A reachable etcd cluster (see the `etcd` role) addressed by `patroni_etcd_hosts`.
- Single-homed nodes so `ansible_default_ipv4.address` is the correct VLAN-30 address, or override `patroni_this_ip`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `patroni_scope` | `authentik` | Patroni cluster scope (the DCS namespace). |
| `patroni_postgres_version` | `16` | PostgreSQL major version. |
| `patroni_data_dir` | `/var/lib/postgresql/{{ patroni_postgres_version }}/main` | PGDATA directory. |
| `patroni_bin_dir` | `/usr/lib/postgresql/{{ patroni_postgres_version }}/bin` | Postgres binaries directory. |
| `patroni_conf_dir` | `/etc/patroni` | Patroni config directory. |
| `patroni_config_file` | `{{ patroni_conf_dir }}/patroni.yml` | Rendered Patroni config path. |
| `patroni_bootstrap_marker` | `{{ patroni_conf_dir }}/.default-cluster-removed` | Marker guarding the one-time drop of the apt-created default cluster. |
| `patroni_venv` | `/opt/patroni` | Virtualenv Patroni is installed into. |
| `patroni_version` | `3.3.2` | Pinned Patroni version. |
| `patroni_lab_cidr` | `""` | CIDR allowed in `pg_hba` for replication and client access. Site data, set by the deploy. |
| `patroni_rest_user` | `patroni` | Patroni REST API username. |
| `patroni_rest_port` | `8008` | Patroni REST API port. |
| `patroni_ttl` | `30` | DCS leader lease TTL (seconds). |
| `patroni_loop_wait` | `10` | Patroni control-loop interval (seconds). |
| `patroni_retry_timeout` | `10` | DCS/Postgres operation retry timeout (seconds). |
| `patroni_max_connections` | `200` | Postgres `max_connections`. |
| `patroni_app_db` | `authentik` | Application database created on the leader. |
| `patroni_app_user` | `authentik` | Application role created on the leader. |
| `patroni_this_ip` | `{{ ansible_default_ipv4.address }}` | This node's address for REST and Postgres listen/advertise. |

Required from inventory (no default): `patroni_etcd_hosts` — the etcd endpoint list for `etcd3.hosts`.

SOPS secrets (no default; must be supplied): `patroni_superuser_password`, `patroni_replication_password`, `patroni_rest_password`, `patroni_app_password`.

## Dependencies

None (no `meta/main.yml`). At runtime this role requires a running etcd cluster (the `etcd` role) reachable via `patroni_etcd_hosts`. The `haproxy_patroni` role routes clients to the leader this role elects.

## What it does

1. Adds the PGDG apt repository and installs `postgresql-16`, the client, `python3-venv`, `python3-pip`, `python3-psycopg2`, and `python3-packaging` (the last is needed by the pip and `community.postgresql` modules and is absent from the minimal image).
2. Installs `patroni[etcd3]` (pinned) plus `psycopg2-binary` into the `patroni_venv` virtualenv.
3. Creates the Patroni config directory (`postgres:postgres`, 0750).
4. Runs `pg_dropcluster --stop 16 main || true` then `touch`es `patroni_bootstrap_marker`, both in one shell step guarded by `creates: {{ patroni_bootstrap_marker }}`. This removes the apt default cluster so Patroni can bootstrap its own at the same `data_dir`, and because the marker is written in the same step a re-run can never drop the cluster Patroni has since created.
5. Stops, disables, and masks the distro `postgresql` service so Patroni is the sole owner of Postgres.
6. Renders `patroni.yml` (0600) and installs the systemd unit, notifying a Patroni restart on change.
7. Enables and starts Patroni.
8. `run_once`: polls the local Patroni REST `/cluster` endpoint (30 retries, 5s apart) until a member has role `leader`.
9. `run_once`: sets `patroni_leader_ip` from the leader's `host`, then connects to that leader to create the application role and database idempotently (`community.postgresql`). These run only against the elected primary, not every replica.

## Example

```yaml
- hosts: patroni
  roles:
    - role: postgres_patroni
      vars:
        patroni_etcd_hosts:
          - 10.0.30.31:2379
          - 10.0.30.32:2379
          - 10.0.20.11:2379   # witness, may sit on another VLAN
        patroni_lab_cidr: 10.0.30.0/24
        # from SOPS:
        patroni_superuser_password: "{{ vault_patroni_superuser_password }}"
        patroni_replication_password: "{{ vault_patroni_replication_password }}"
        patroni_rest_password: "{{ vault_patroni_rest_password }}"
        patroni_app_password: "{{ vault_patroni_app_password }}"
```

## Notes

- Failover timing invariant: `ttl` must be `>= loop_wait + 2*retry_timeout`. The defaults (30, 10, 10) sit exactly on that bound; keep the relationship if you change any of them or the leader lease can expire mid-loop and cause spurious failovers.
- Patroni owns Postgres. The distro `postgresql` service is masked deliberately; do not unmask or start it, and never edit PGDATA out from under Patroni.
- The default-cluster drop is one-time by design. The marker file is what makes it safe on re-runs; do not delete it on a bootstrapped node.
- The application role and database are created once, on the leader only, via the REST-discovered `patroni_leader_ip`. Replicas receive them through streaming replication.
