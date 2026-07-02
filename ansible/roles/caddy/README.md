# caddy

Manages the Caddyfile and internal CA trust for an existing Caddy reverse proxy; the Caddy binary itself is installed elsewhere.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu on the target (systemd, `update-ca-certificates`).
- Caddy already installed and running as a systemd service. In this library Caddy is provisioned by Terraform cloud-init on the incus instance (in the deploy repo); this role owns only the config and trust store, so a route change is an Ansible run plus reload, not an instance rebuild.
- The Caddy binary must include the `dns.providers.route53` module (the template uses `tls { dns route53 }` for a wildcard cert), and Route53 credentials must be available to the running service.
- Ansible `ansible.builtin` only. No external collections.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `caddy_domain` | `cypherworks.co.uk` | Base domain; the site block serves `*.{{ caddy_domain }}` and routes by Host header. |
| `caddy_acme_email` | `lloyd@cypherworks.co.uk` | ACME account email in the global block. |
| `caddy_config_path` | `/etc/caddy/Caddyfile` | Where the Caddyfile is written. |
| `caddy_binary` | `/usr/local/bin/caddy` | Caddy binary used for `validate` and `reload`. |
| `caddy_routes` | `[]` | Site data. Declarative list of routes (see below). |
| `caddy_trusted_ca_certs` | `{}` | Site data. Map of name to PEM content, installed into the system trust store so routes can verify internal HTTPS upstreams. |

Each `caddy_routes` entry: `name` (matcher id), `host` (FQDN), and either `upstream` (single backend) or `upstreams` (a list, load-balanced). Optional: `lb_policy` (defaults to `round_robin` when more than one upstream), `health_uri` and `health_interval` (default `10s`) for active health checks, `tls_skip_verify` for self-signed backends, `header_up` (a map of headers to set upstream), and `header_up_remove` (a list of headers to strip).

## Dependencies

None.

## What it does

1. Installs each entry of `caddy_trusted_ca_certs` to `/usr/local/share/ca-certificates/<name>.crt` (`0644`). A change notifies both `Update CA trust` and `Restart Caddy`.
2. Renders the Caddyfile to `caddy_config_path` (`0644`) from `Caddyfile.j2`: one `*.{{ caddy_domain }}` site with a Route53 DNS-01 wildcard cert, a `handle` block per route, and a default `respond "lab ingress" 200`. Notifies `Reload Caddy`.
3. Runs `caddy validate --adapter caddyfile` against the rendered file (`changed_when: false`). This gates the change: a bad config fails the play before any handler runs, so Caddy keeps serving the previous config.

Handlers, ordered deliberately:
- `Update CA trust` runs `update-ca-certificates`. Defined before `Reload Caddy` so the store is refreshed first.
- `Reload Caddy` does a graceful, sub-second config swap (`caddy reload`).
- `Restart Caddy` does a full systemd restart, only for a trust-store change. Caddy caches the system cert pool at startup, so a graceful reload keeps the stale pool and a new CA would never be trusted. Defined last so if a run also changed the Caddyfile, the reload runs first and the restart lands on the current config.

## Example

```yaml
- hosts: ingress
  roles:
    - role: caddy
      vars:
        caddy_routes:
          - name: unifi
            host: unifi.cypherworks.co.uk
            upstream: 10.200.30.20:8443
            tls_skip_verify: true
            header_up:
              Host: "{http.reverse_proxy.upstream.hostport}"
          - name: bao
            host: bao.cypherworks.co.uk
            upstream: https://10.200.30.30:8200
        caddy_trusted_ca_certs:
          openbao-internal: "{{ openbao_ca_pem }}"
```

## Notes

- This role does not install or update Caddy. If the deployed binary lacks the Route53 DNS module, the wildcard cert block fails at runtime even though `caddy validate` passes.
- Trust changes require the full restart (brief connection drop). Ordinary route changes reload gracefully with no dropped connections.
- Prefer trusting an internal CA via `caddy_trusted_ca_certs` over `tls_skip_verify`, so upstream identity is actually verified.
