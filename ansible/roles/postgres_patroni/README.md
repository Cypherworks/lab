# postgres_patroni

Installs PostgreSQL 16 (PGDG) managed by Patroni, with automatic leader election and failover through an etcd DCS.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu target with systemd; a minimal cloud image is fine (the role installs the Python prerequisites it needs).
- The `community.postgresql` collection on the controller (used to create the application role and database).
- OpenBao PKI and the `openbao_cert` role for each member's server cert. The deploy sets `openbao_cert_bao_addr`, `openbao_cert_pki_role`, `openbao_cert_ssm_param` and `openbao_cert_aws_env`; this role sets the cert's name, files and SANs.
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
| `patroni_members` | `[]` | The members as `[{name, ip}]`. Each may log in as `postgres` and `replicator`, over TLS, from its own address. Site data. |
| `patroni_databases` | `[]` | Each app's database as `[{name, owner, password, client_ips, conn_limit}]`. The leader creates `owner` with `password` (from SOPS) and `conn_limit` (optional; unset is no limit), then `name` owned by it. `owner` may log in to `name` only, over TLS, from each `client_ips` address. Site data. |
| `patroni_tls_service_name` | `""` | DNS name in every member's cert, for clients that dial the database by name. Each cert also carries its member's IP. |
| `patroni_tls_dir` | `{{ patroni_conf_dir }}/tls` | Holds `postgres.crt`, `postgres.key` and `postgres-ca.crt`. |
| `patroni_rest_user` | `patroni` | Patroni REST API username. |
| `patroni_rest_port` | `8008` | Patroni REST API port. |
| `patroni_ttl` | `30` | DCS leader lease TTL (seconds). |
| `patroni_loop_wait` | `10` | Patroni control-loop interval (seconds). |
| `patroni_retry_timeout` | `10` | DCS/Postgres operation retry timeout (seconds). |
| `patroni_max_connections` | `200` | Postgres `max_connections`. |
| `patroni_this_ip` | `{{ ansible_default_ipv4.address }}` | This node's address for REST and Postgres listen/advertise. |

Required from inventory (no default): `patroni_etcd_hosts` — the etcd endpoint list for `etcd3.hosts`.

SOPS secrets (no default; must be supplied): `patroni_superuser_password`, `patroni_replication_password`, `patroni_rest_password`, and each database's `password`.

## Dependencies

None (no `meta/main.yml`). At runtime this role requires a running etcd cluster (the `etcd` role) reachable via `patroni_etcd_hosts`. The `haproxy_patroni` role routes clients to the leader this role elects.

## What it does

1. Adds the PGDG apt repository and installs `postgresql-16`, the client, `python3-venv`, `python3-pip`, `python3-psycopg2`, and `python3-packaging` (the last is needed by the pip and `community.postgresql` modules and is absent from the minimal image).
2. Installs `patroni[etcd3]` (pinned) plus `psycopg2-binary` into the `patroni_venv` virtualenv.
3. Creates the Patroni config directory (`postgres:postgres`, 0750).
4. Runs `pg_dropcluster --stop 16 main || true` then `touch`es `patroni_bootstrap_marker`, both in one shell step guarded by `creates: {{ patroni_bootstrap_marker }}`. This removes the apt default cluster so Patroni can bootstrap its own at the same `data_dir`, and because the marker is written in the same step a re-run can never drop the cluster Patroni has since created.
5. Stops, disables, and masks the distro `postgresql` service so Patroni is the sole owner of Postgres.
6. Issues the member's server cert through `openbao_cert` (CN and DNS SAN `patroni_tls_service_name`, IP SAN the member's address). It's reissued only when due, so a re-run leaves it alone.
7. Renders `patroni.yml` (0600) and installs the systemd unit. A config or cert change reloads Patroni, which applies `pg_hba`, TLS and parameter changes with no failover; only a unit change restarts it. The handlers run before the next step.
8. Enables and starts Patroni, then waits for Postgres to answer over TLS, verified against its own address.
9. `run_once`: polls the local Patroni REST `/cluster` endpoint (30 retries, 5s apart) until a member has role `leader`.
10. `run_once`: sets `patroni_leader_ip` from the leader's `host`, then connects to that leader with `verify-full` to create each database's owner and the database idempotently (`community.postgresql`). These run only against the elected primary, not every replica.

## Access

Patroni writes the whole `pg_hba.conf` from `postgresql.pg_hba` on every start and reload, so the file always matches the role. It admits only:

- `local all postgres peer`, for an operator on the member as the `postgres` OS user;
- `hostssl all postgres` and `hostssl replication replicator` from each member's `/32`, for Patroni, `pg_rewind`, base backups and streaming;
- `hostssl <name> <owner>` from each of a database's `client_ips` `/32`, for each of `patroni_databases`.

Every host line is `scram-sha-256`, and the server's minimum is TLS 1.3. Patroni's own superuser and replication connections use `sslmode=verify-full` against `postgres-ca.crt`, dialling members by IP, which each cert carries.

## Example

```yaml
- hosts: patroni
  roles:
    - role: postgres_patroni
      vars:
        patroni_etcd_hosts:
          - 10.0.30.31:2379
          - 10.0.30.32:2379
          - 10.0.30.33:2379
        patroni_members:
          - { name: pg-1, ip: 10.0.30.21 }
          - { name: pg-2, ip: 10.0.30.22 }
        patroni_databases:
          - { name: app, owner: app, password: "{{ vault_app_db_password }}", client_ips: [10.0.30.11, 10.0.30.12], conn_limit: 50 }
        patroni_tls_service_name: db.example.com
        openbao_cert_bao_addr: https://10.0.30.40:8200
        openbao_cert_pki_role: infra
        openbao_cert_ssm_param: /openbao/provisioner
        # from SOPS:
        patroni_superuser_password: "{{ vault_patroni_superuser_password }}"
        patroni_replication_password: "{{ vault_patroni_replication_password }}"
        patroni_rest_password: "{{ vault_patroni_rest_password }}"
```

## Notes

- Failover timing invariant: `ttl` must be `>= loop_wait + 2*retry_timeout`. The defaults (30, 10, 10) sit exactly on that bound; keep the relationship if you change any of them or the leader lease can expire mid-loop and cause spurious failovers.
- Patroni owns Postgres. The distro `postgresql` service is masked deliberately; do not unmask or start it, and never edit PGDATA out from under Patroni.
- The default-cluster drop is one-time by design. The marker file is what makes it safe on re-runs; do not delete it on a bootstrapped node.
- The application role and database are created once, on the leader only, via the REST-discovered `patroni_leader_ip`. Replicas receive them through streaming replication.
- A parameter that needs a restart (e.g. `max_connections`) is left pending after a reload; Patroni reports `pending_restart` until you restart the member, one at a time.
- The Patroni REST API (8008) is still plain HTTP with basic auth.

## Tests

Both need Docker, git, the AWS CLI and internet for images, debs and pip, and can't run at the same time. From `ansible/`:

- `tests/tls.yml` builds a cluster with the role as it was before hardening, then applies this one while an app writes on the default `sslmode`. It checks no write fails, TLS 1.3 is on, `pg_hba` is exactly the list above, verify-full works by name and every other way in is refused, Patroni and replication use TLS, a new cert is served without a restart, a switchover streams over TLS, and a re-run changes nothing.
- `tests/bootstrap.yml` builds a cluster from nothing with this role and checks the replica streams over TLS, the app logs in by name and a re-run changes nothing.

```sh
ANSIBLE_ROLES_PATH=roles ansible-playbook -i localhost, roles/postgres_patroni/tests/tls.yml
ANSIBLE_ROLES_PATH=roles ansible-playbook -i localhost, roles/postgres_patroni/tests/bootstrap.yml
```
