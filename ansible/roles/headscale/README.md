# headscale

Headscale control server installed from a pinned upstream `.deb`, with an embedded DERP relay and optional Authentik OIDC.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu on the target (apt, systemd). Installs from a downloaded `.deb`, which creates the `headscale` service and user.
- Ansible `ansible.builtin` only. No external collections.
- Outbound HTTPS to `github.com` to fetch the pinned release. Defaults ship an `arm64` package; set `headscale_arch: amd64` for x86.
- Reachability for a TLS-ALPN-01 cert on 443 (standalone) or an SNI passthrough front (the `sni_router` role) when `headscale_listen_addr` is a local port.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `headscale_version` | `0.26.1` | Pinned release. `>= 0.26` carries the HA subnet-router failover fix (LP #2228). |
| `headscale_arch` | `arm64` | Package architecture. Use `amd64` on x86 nodes. |
| `headscale_release_base` | `https://github.com/juanfont/headscale/releases/download/v{{ headscale_version }}` | Release URL base. |
| `headscale_deb_name` | `headscale_{{ headscale_version }}_linux_{{ headscale_arch }}.deb` | Package filename. |
| `headscale_deb_url` | `{{ headscale_release_base }}/{{ headscale_deb_name }}` | Full `.deb` download URL. |
| `headscale_checksum` | `sha256:{{ headscale_release_base }}/checksums.txt` | Integrity check for the `.deb`. A `sha256:<url>` verifies against Headscale's published checksums file; override with a literal `sha256:<hash>` to pin, or `""` to skip. |
| `headscale_domain` | `headscale.example.com` | Public control-plane hostname (`server_url` and the TLS-ALPN cert). |
| `headscale_listen_addr` | `0.0.0.0:443` | Listen address. Behind the EC2 `sni_router`, override to a local port (e.g. `127.0.0.1:8443`). |
| `headscale_metrics_listen_addr` | `127.0.0.1:9090` | Prometheus `/metrics` bind. Override to the host's tailnet IP to scrape over the overlay. Never public. |
| `headscale_metrics_nonlocal_bind` | `false` | Set `true` when `headscale_metrics_listen_addr` is this host's own tailnet IP: that address only exists after the host joins the overlay Headscale serves, so it must be allowed to bind before the address is up. See notes. |
| `headscale_base_domain` | `ts.example.com` | MagicDNS suffix; a tailnet-only subdomain so it never clashes with the real Route53 zone. |
| `headscale_acme_email` | `admin@example.com` | ACME contact email. |
| `headscale_oidc_enabled` | `false` | Whether to render the OIDC block. |
| `headscale_oidc_only_start_if_available` | `false` | Deliberately false. See notes on the boot deadlock. |
| `headscale_oidc_issuer` | `""` | Required when OIDC enabled, from inventory. Authentik issuer URL. |
| `headscale_oidc_client_id` | `""` | Required when OIDC enabled. OIDC client id. |
| `headscale_oidc_client_secret` | `""` | Required when OIDC enabled, from SOPS. OIDC client secret. |
| `headscale_oidc_allowed_groups` | `["lab-admins"]` | Authentik groups permitted to obtain the lab routes. |
| `headscale_oidc_scopes` | `["openid", "profile", "email"]` | Requested scopes. Deliberately excludes `groups`. See notes. |
| `headscale_selfjoin_user` | `""` | When set, the role ensures this Headscale user exists and resolves its numeric id into `headscale_selfjoin_user_id`, so the host can mint its own pre-auth key and self-join. Empty on standalone control planes. |

## Dependencies

None.

## What it does

1. Downloads the pinned `.deb` to `/tmp/{{ headscale_deb_name }}` (`0644`), verified against `headscale_checksum`.
2. Installs it with apt, which creates the `headscale` systemd service and the `headscale` group.
3. Renders `/etc/headscale/config.yaml` (`0640`, group `headscale`) from `config.yaml.j2`: `server_url`, listen/metrics/gRPC addresses, IPv4/IPv6 tailnet prefixes, a self-hosted embedded DERP relay (region 999, STUN on `0.0.0.0:3478`, no public Tailscale DERP), a sqlite database, a TLS-ALPN-01 Let's Encrypt cert on 443, MagicDNS, and (only when `headscale_oidc_enabled`) the OIDC block.
4. When `headscale_metrics_nonlocal_bind`, sets `net.ipv4.ip_nonlocal_bind=1` so the metrics address can be bound before it exists on an interface.
5. Probes the listen port and enables the service, restarting it when the config changed, the sysctl changed, or nothing is listening, otherwise just ensuring it is started; then `wait_for`s the port.
6. When `headscale_selfjoin_user` is set, ensures that user exists (tolerating "already exists") and resolves its numeric id into `headscale_selfjoin_user_id`.

Handlers: none — the running service is reconciled to the on-disk config directly in tasks (see Notes).

## Example

```yaml
- hosts: overlay_control
  roles:
    - role: headscale
      vars:
        headscale_arch: arm64
        headscale_listen_addr: "127.0.0.1:8443"   # behind sni_router
        headscale_metrics_listen_addr: "100.64.0.1:9090"
        headscale_oidc_enabled: true
        headscale_oidc_issuer: "https://auth.example.com/application/o/headscale/"
        headscale_oidc_client_id: "{{ vault_hs_client_id }}"
        headscale_oidc_client_secret: "{{ vault_hs_client_secret }}"
```

## Notes

- `headscale_oidc_only_start_if_available` is false on purpose. The EC2 reaches Authentik over the very overlay Headscale runs, so a hard OIDC-at-startup dependency deadlocks (Authentik blip -> Headscale won't start -> overlay degrades -> can't reach Authentik). Starting anyway breaks the loop; OIDC enrolment resumes once Authentik is reachable.
- `headscale_oidc_scopes` deliberately omits `groups`: that scope triggers Headscale's "empty OIDC callback params" failure (headscale#2887). Authentik's `profile` scope already carries the groups claim that `allowed_groups` matches against.
- `headscale_metrics_listen_addr` must never bind `0.0.0.0` or the public interface. Keep it on loopback or the tailnet IP; the EC2 security group stays closed and the tailnet is the only path in.
- The `.deb` is verified against Headscale's published checksums file by default (`headscale_checksum`). This catches transport corruption and tampering of the artifact; for a stronger guarantee against a compromised release, override `headscale_checksum` with a literal `sha256:<hash>` pinned to a reviewed version.
- With OIDC off, bootstrap routers and the laptop with short-lived pre-auth keys issued by hand.
- `headscale_metrics_nonlocal_bind` breaks a boot cycle specific to a host that joins its own overlay: `metrics_listen_addr` is that host's tailnet IP, which only appears once it joins, but Headscale binds the address at startup and would crash-loop on `cannot assign requested address`. Allowing non-local bind lets it claim the address early; nothing can reach it until the overlay is up anyway.
- The service is reconciled directly rather than via a notify handler. A handler is deferred to the end of the play, so an interrupted run (or a later role that dials Headscale) would see a service still on its install-time config. Restarting on "config/sysctl changed or port not listening" converges from any state.
- `headscale_selfjoin_user` supports a host that is its own control server: the role ensures the user and exposes its id, and the `tailscale` role's `tailscale_authkey_command` mints a fresh key against the live database at join time — no perishable key in SOPS.
