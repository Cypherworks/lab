# authentik

Deploys the standalone Authentik identity provider (server, worker, and bundled PostgreSQL) from the official docker-compose stack.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Docker Engine and the Compose v2 plugin on the host, plus the `community.docker` collection.
- `authentik_secret_key` and `authentik_pg_password` supplied from SOPS (the role asserts both are set).
- Caddy (or equivalent) terminating TLS and reverse-proxying to the HTTP port; Authentik trusts the `X-Forwarded-*` headers.

## Role variables

| Variable | Default | Description |
| --- | --- | --- |
| `authentik_image` | `ghcr.io/goauthentik/server` | Upstream server image (also used for the worker). |
| `authentik_tag` | `2026.5.3` | Pinned image tag; bump deliberately. |
| `authentik_data_dir` | `/opt/authentik` | Host directory for the compose file, `.env`, and bind-mounts. |
| `authentik_pg_user` | `authentik` | Postgres user for the bundled database. |
| `authentik_pg_db` | `authentik` | Postgres database name. |
| `authentik_http_port` | `9000` | HTTP listen port (Caddy proxies to this). |
| `authentik_https_port` | `9443` | HTTPS listen port. |
| `authentik_secret_key` | `""` | Authentik secret key (from SOPS; no default, required). |
| `authentik_pg_password` | `""` | Postgres password (from SOPS; no default, required). |

## Dependencies

None declared in `meta/main.yml`; the host must already have Docker and Compose v2 (unlike `authentik_app`, this role does not pull in the `docker` role).

## What it does

1. Asserts `authentik_secret_key` and `authentik_pg_password` are set.
2. Creates the data directory and the `data`, `certs`, and `custom-templates` bind-mount directories.
3. Renders the `.env` (secret key + DB password, `no_log`) and the compose file.
4. Deploys the stack (server, worker, bundled PostgreSQL) with `docker_compose_v2`, pulling missing images.

## Example

```yaml
- hosts: authentik
  become: true
  roles:
    - role: authentik
      vars:
        authentik_tag: "2026.5.3"
        authentik_secret_key: "{{ vault_authentik_secret_key }}"
        authentik_pg_password: "{{ vault_authentik_pg_password }}"
```

## Notes

- This is the standalone, bundled-database Authentik (server + worker + its own PostgreSQL), distinct from `authentik_app`, which runs the app tier only against external HA Postgres/Redis.
- The image tag is pinned for reproducibility; verify the tag exists in the registry before bumping.
- TLS is terminated upstream by Caddy with a wildcard cert; the container speaks plain HTTP on `authentik_http_port`.
