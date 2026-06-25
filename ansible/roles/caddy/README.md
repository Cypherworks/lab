# caddy

Manages the Caddy reverse-proxy ingress config. Caddy is installed by the
Terraform `incus` stack (cloud-init); this role owns the `Caddyfile`, so adding or
removing a service is an Ansible run plus a sub-second reload, never an instance
rebuild.

## What it does

- Renders the `Caddyfile` from `caddy_routes` — one wildcard site (Route53 DNS-01
  cert) that routes by `Host` header to each backend.
- Validates the config, then reloads Caddy gracefully (only if valid).

## Adding / removing a service

Edit `caddy_routes` (in group_vars) and run the play:

```yaml
caddy_routes:
  - { name: dsm,   host: "dsm.cypherworks.co.uk",   upstream: "https://10.200.20.30:5001", tls_skip_verify: true }
  - { name: unifi, host: "unifi.cypherworks.co.uk", upstream: "https://10.200.20.30:8443", tls_skip_verify: true }
```

| Variable | Default | Purpose |
| --- | --- | --- |
| `caddy_domain` | `cypherworks.co.uk` | Zone for the wildcard cert |
| `caddy_acme_email` | lloyd@… | ACME account email |
| `caddy_routes` | `[]` | Host → upstream routes (set in group_vars) |
| `caddy_config_path` | `/etc/caddy/Caddyfile` | Caddyfile path |
| `caddy_binary` | `/usr/local/bin/caddy` | Caddy binary |

`tls_skip_verify: true` is for backends with self-signed certs (DSM, UniFi).

## Connection

The Caddy instance is reached over the Incus connection (`incus exec`), so it
needs no SSH server of its own — see the inventory in homelab-deploy.
