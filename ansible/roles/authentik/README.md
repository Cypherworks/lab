# authentik

Deploys [Authentik](https://goauthentik.io) — the lab's OIDC identity provider —
as the official upstream docker-compose stack (server + worker + PostgreSQL) on a
Docker host. It's the MFA gate for the Headscale overlay: device enrolment
redirects here, and only members of an allowed group are authorised.

> The current upstream compose (2026.5.x) no longer ships a separate Redis
> container, so this role doesn't deploy one — it reproduces the official file as-is.

## What it does

- Renders `{{ authentik_data_dir }}/compose.yaml` (verbatim upstream) and a `.env`
  (0600) with the pinned image tag, ports, and secrets.
- Brings the stack up with `community.docker.docker_compose_v2` (`pull: missing`).

Runs over the Incus connection as root (the `services` group); the host is a VM
with Docker installed by cloud-init (`terraform/incus`).

## Secrets (from SOPS — no defaults)

| Variable | How to generate |
| --- | --- |
| `authentik_secret_key` | `openssl rand -base64 60 \| tr -d '\n'` |
| `authentik_pg_password` | `openssl rand -base64 36 \| tr -d '\n'` |

## Key variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `authentik_image` | `ghcr.io/goauthentik/server` | Server/worker image |
| `authentik_tag` | `2026.5.3` | Pinned version |
| `authentik_data_dir` | `/opt/authentik` | Compose + bind-mounts |
| `authentik_http_port` | `9000` | HTTP (Caddy proxies here) |
| `authentik_https_port` | `9443` | HTTPS (direct, self-signed) |

## Ingress

Caddy terminates TLS with the wildcard cert and proxies `auth.cypherworks.co.uk`
to the HTTP port — Authentik trusts the `X-Forwarded-*` headers. Reachable from
abroad via the EC2 SNI passthrough → Caddy → here.
