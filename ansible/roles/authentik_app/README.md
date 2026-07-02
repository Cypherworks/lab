# authentik_app

Deploys the Authentik app tier (server and worker only) against external HA Postgres/Redis via node-local HAProxy, renders eight integration blueprints, and ships an LDAP outpost.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Docker (pulled in via the `docker` meta dependency) and the `community.docker` collection.
- External HA PostgreSQL and Redis reachable through the node-local HAProxy frontends (Postgres on `authentik_pg_port`, Redis via the role:master frontend on `authentik_redis_port`).
- Secrets from SOPS: `authentik_secret_key`, `authentik_pg_password`, `authentik_redis_password`, `authentik_bootstrap_password`, the per-integration OIDC client id/secret pairs, and `ldap_outpost_token`.
- Caddy load-balancing the server instances across cluster members.

## Role variables

| Variable | Default | Description |
| --- | --- | --- |
| `authentik_image` | `ghcr.io/goauthentik/server:2025.6.4` | Server/worker image; pin to a confirmed current stable before apply. |
| `authentik_ldap_outpost_image` | `ghcr.io/goauthentik/ldap:2025.6.4` | LDAP outpost image; must track the server version. |
| `authentik_compose_dir` | `/opt/authentik` | Host directory for the compose file, `.env`, and bind-mounts. |
| `authentik_server_port` | `9000` | Server HTTP port. |
| `authentik_pg_host` | `{{ ansible_default_ipv4.address }}` | Postgres host — the node's lab IP (HAProxy frontend), not loopback. |
| `authentik_pg_port` | `5000` | Postgres HAProxy frontend port. |
| `authentik_pg_name` | `authentik` | Postgres database name. |
| `authentik_pg_user` | `authentik` | Postgres user. |
| `authentik_redis_host` | `{{ ansible_default_ipv4.address }}` | Redis host — the node's lab IP (HAProxy role:master frontend). |
| `authentik_redis_port` | `6379` | Redis HAProxy frontend port. |
| `authentik_blueprints_dir` | `{{ authentik_compose_dir }}/blueprints` | Where blueprints are rendered; mounted read-only at `/blueprints/custom`. |
| `authentik_media_dir` | `{{ authentik_compose_dir }}/media` | Media dir mounted at `/media` for served assets. |
| `authentik_branding_files` | `[]` | Branding files copied into `<media>/public` (site data). |
| `authentik_branding_logo_path` | `""` | Brand logo path, e.g. `/media/public/<name>` (site data). |
| `authentik_branding_favicon_path` | `""` | Brand favicon path (site data). |
| `authentik_login_title` | `""` | Login card heading (default flow title); empty keeps the stock text. |
| `headscale_oidc_redirect_uri` | `""` | Headscale OIDC callback (site data). |
| `nas_oidc_redirect_uris` | `[]` | NAS (DSM) OIDC base URLs, internal + external (site data). |
| `grafana_oidc_redirect_uri` | `""` | Grafana `generic_oauth` callback (site data). |
| `authentik_admin_email` | `""` | Email set on the bootstrap superuser (akadmin) by the core blueprint (site data). |
| `authentik_ldap_base_dn` | `""` | Base DN the LDAP outpost serves (site data). |
| `authentik_ldap_outpost_host` | `""` | Public Authentik URL the outpost dials back on (site data). |
| `ldap_outpost_token` | `""` | Outpost service-account token / `AUTHENTIK_TOKEN` (from SOPS). |
| `ldap_search_password` | `""` | App-password for the `ldap-search` bind account (SSSD bind credential; from SOPS). |
| `ldaps_cert` | `""` | LDAPS server cert (PEM) for the outpost's 6636 listener (from SOPS); empty = self-signed. |
| `ldaps_key` | `""` | LDAPS server key (PEM) (from SOPS). |
| `authentik_secret_key` | `""` | Authentik secret key (from SOPS). |
| `authentik_pg_password` | `""` | Postgres password (from SOPS). |
| `authentik_redis_password` | `""` | Redis password (from SOPS). |
| `authentik_bootstrap_password` | `""` | Bootstrap superuser password (from SOPS). |
| `headscale_oidc_client_id` | `""` | Headscale OIDC client id (from SOPS; blueprint reads via `!Env`). |
| `headscale_oidc_client_secret` | `""` | Headscale OIDC client secret (from SOPS). |
| `nas_oidc_client_id` | `""` | NAS OIDC client id (from SOPS). |
| `nas_oidc_client_secret` | `""` | NAS OIDC client secret (from SOPS). |
| `grafana_oidc_client_id` | `""` | Grafana OIDC client id (from SOPS). |
| `grafana_oidc_client_secret` | `""` | Grafana OIDC client secret (from SOPS). |
| `openbao_oidc_client_id` | `""` | OpenBao OIDC client id (from SOPS; blueprint reads via `!Env`). |
| `openbao_oidc_client_secret` | `""` | OpenBao OIDC client secret (from SOPS). |
| `openbao_oidc_redirect_uris` | `[]` | OpenBao UI + CLI callbacks; must match the openbao role's `allowed_redirect_uris` (site data). |

## Dependencies

`docker` (via `meta/main.yml`) — installs Docker Engine and Compose v2.

## What it does

1. Creates the compose, blueprints, and media directories.
2. Copies branding assets into `<media>/public`.
3. Renders the eight integration blueprints into `authentik_blueprints_dir`: `core`, `grafana`, `headscale`, `invitation`, `ldap`, `nas`, `openbao`, `recovery`.
4. Renders the `.env` and compose file.
5. Starts the stack (server + worker) with `docker_compose_v2`, pulling missing images.

## Example

```yaml
- hosts: authentik_app
  become: true
  roles:
    - role: authentik_app
      vars:
        authentik_secret_key: "{{ vault_authentik_secret_key }}"
        authentik_pg_password: "{{ vault_authentik_pg_password }}"
        authentik_redis_password: "{{ vault_authentik_redis_password }}"
        authentik_bootstrap_password: "{{ vault_authentik_bootstrap_password }}"
        authentik_admin_email: admin@example.com
        headscale_oidc_redirect_uri: "https://headscale.example.com/oidc/callback"
        headscale_oidc_client_id: "{{ vault_headscale_oidc_client_id }}"
        headscale_oidc_client_secret: "{{ vault_headscale_oidc_client_secret }}"
        openbao_oidc_client_id: "{{ vault_openbao_oidc_client_id }}"
        openbao_oidc_client_secret: "{{ vault_openbao_oidc_client_secret }}"
        openbao_oidc_redirect_uris:
          - "https://openbao.example.com/ui/vault/auth/oidc/oidc/callback"
        authentik_ldap_base_dn: "dc=example,dc=com"
        authentik_ldap_outpost_host: "https://authentik.example.com"
        ldap_outpost_token: "{{ vault_ldap_outpost_token }}"
```

## Notes

- This is the app tier only (server + worker). The HA Postgres/Redis data tier is external, reached via the node-local HAProxy frontends on the node's lab IP (not loopback, because the containers run on the compose bridge, not host networking). Redis is a single endpoint — Authentik has no Sentinel support, so Sentinel drives the failover behind HAProxy.
- Blueprints are discovered at worker startup, not via a live file-watch, so a re-rendered blueprint is only picked up on restart — hence every render notifies `Restart authentik`. Add an integration by dropping `templates/blueprints/<name>.yaml.j2` and adding `<name>` to the render loop.
- The outpost image must track the server version — the outpost protocol is tied to the core release. Pin the same tag as `authentik_image`.
- Blueprints read OIDC creds via `!Env` from the `.env`, keeping one source of truth shared with the consuming roles' own SOPS-sourced config.
