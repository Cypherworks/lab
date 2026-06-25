# headscale

Installs and configures the Headscale control plane on the EC2 host (built by
`terraform/headscale`). Run after `base` + `UBUNTU24-CIS` so the box is hardened
like the rest of the estate.

## What it does

- Installs a pinned Headscale (≥ 0.26 — where HA subnet-router failover was fixed).
- Renders `/etc/headscale/config.yaml`: HTTPS on 443 with a TLS-ALPN-01 cert (no
  port 80), an embedded DERP relay (STUN 3478, no public Tailscale DERP), SQLite,
  and MagicDNS on a tailnet-only base domain.
- Enables the service.

## Auth model

- `headscale_oidc_enabled: false` (default) — bootstrap with short-lived pre-auth
  keys issued by hand (`headscale preauthkeys create`) for the routers and the
  laptop.
- Flip to `true` once Authentik + the EC2 passthrough exist: enrolment then
  redirects to Authentik, which enforces **MFA**, and only `allowed_groups` members
  get authorised. That's the gate on the initial connection.

## Routes / HA

Routers advertise `10.200.0.0/16`; approve with `headscale nodes` / route commands
(manual, no auto-approve). Two routers advertising the same prefix give HA failover.

| Variable | Default | Purpose |
| --- | --- | --- |
| `headscale_version` | `0.26.1` | Pinned release |
| `headscale_domain` | headscale.cypherworks.co.uk | Public control-plane URL |
| `headscale_base_domain` | ts.cypherworks.co.uk | MagicDNS suffix (tailnet-only) |
| `headscale_oidc_enabled` | `false` | Switch from pre-auth to OIDC/MFA |
