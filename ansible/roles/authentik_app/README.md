# authentik_app

The Authentik **app tier** — `server` + `worker` only — deployed via the official
docker-compose on each cluster member (Caddy load-balances the two servers). The
HA data tier (Patroni Postgres, Redis+Sentinel) is **external**; the app reaches it
through the node-local `haproxy_patroni` frontends on the node's lab IP at `:5000`
(Postgres leader) and `:6379` (Redis master). The containers run on the compose
bridge (not host networking — server and worker would collide on `:9000`); only the
server's `:9000` is published for Caddy.

## Why compose, not Incus-native OCI

Authentik's only upstream-tested deploy is docker-compose. Running its multi-process
image as bare Incus OCI application containers is unverified, and this gates the
whole auth layer — so we stay on the supported path (server+worker in a nesting
container per node) while still distributing across tc1/tc2 for HA.

## Why a Redis HAProxy frontend

**Authentik does not support Redis Sentinel** — it only accepts a single
`AUTHENTIK_REDIS__HOST`. So `haproxy_patroni` runs a `role:master` Redis frontend
on each app node; Sentinel still drives failover, Authentik just uses the local
endpoint. See `haproxy_patroni`'s `haproxy_redis_backends`.

## Blueprints (config as code)

The Authentik configuration (providers, applications, groups) is declared as
[blueprints](https://docs.goauthentik.io/customize/blueprints/), rendered into
`authentik_blueprints_dir` and mounted read-only at `/blueprints/custom`; authentik
file-watches the dir and reconciles every YAML in a single atomic transaction.
`AUTHENTIK_BOOTSTRAP_PASSWORD` sets `akadmin` on first boot, so there's no manual
initial-setup.

**Add an integration:** drop a template under `templates/blueprints/<app>.yaml.j2`
and a render task mirroring the Headscale one. Secrets are read by the blueprint via
`!Env [VAR]` from the `.env`, so a credential lives in one place that both Authentik
and the consuming service read.

**MFA enforcement is NOT auto-applied** (it modifies the live authentication flow —
a wrong stage blueprint can lock login). It's documented as a verify-then-apply step
in the deploy, not a rendered file.

## Secrets (SOPS)

`authentik_secret_key`, `authentik_pg_password`, `authentik_redis_password`,
`authentik_bootstrap_password`, `headscale_oidc_client_id`,
`headscale_oidc_client_secret`.

## Pin the image

`authentik_image` defaults to a placeholder — set it to a confirmed current stable
tag before applying.
