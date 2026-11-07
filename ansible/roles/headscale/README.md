# headscale

Headscale control server installed from a pinned upstream `.deb`, with an embedded DERP relay and optional Authentik OIDC.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu on the target (apt, systemd). Installs from a downloaded `.deb`, which creates the `headscale` service and user.
- Ansible `ansible.builtin` plus `ansible.posix` (the metrics non-local-bind step uses `ansible.posix.sysctl`).
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
| `headscale_metrics_listen_addr` | `127.0.0.1:9090` | Bind for Headscale's debug server, which serves `/metrics` and `/debug/*`. Keep it on loopback and scrape locally. See notes. |
| `headscale_metrics_nonlocal_bind` | `false` | Set `true` when `headscale_metrics_listen_addr` is this host's own tailnet IP: that address only exists after the host joins the overlay Headscale serves, so it must be allowed to bind before the address is up. See notes. |
| `headscale_base_domain` | `ts.example.com` | MagicDNS suffix; a tailnet-only subdomain so it never clashes with the real Route53 zone. |
| `headscale_dns_nameservers` | `["1.1.1.1", "1.0.0.1"]` | DNS servers pushed to `accept-dns` clients. |
| `headscale_dns_override_local` | `false` | Force `accept-dns` clients onto those servers, overriding local DNS. |
| `headscale_acme_email` | `admin@example.com` | ACME contact email. |
| `headscale_oidc_enabled` | `false` | Whether to render the OIDC block. |
| `headscale_oidc_only_start_if_available` | `false` | Deliberately false. See notes on the boot deadlock. |
| `headscale_oidc_issuer` | `""` | Required when OIDC enabled, from inventory. Authentik issuer URL. |
| `headscale_oidc_client_id` | `""` | Required when OIDC enabled. OIDC client id. |
| `headscale_oidc_client_secret` | `""` | Required when OIDC enabled, from SOPS. OIDC client secret. |
| `headscale_oidc_allowed_groups` | `["lab-admins"]` | Authentik groups permitted to obtain the lab routes. |
| `headscale_oidc_scopes` | `["openid", "profile", "email"]` | Requested scopes. Deliberately excludes `groups`. See notes. |
| `headscale_oidc_watchdog_on_boot` | `2min` | Delay after boot before the OIDC self-heal watchdog timer first fires. |
| `headscale_oidc_watchdog_interval` | `5min` | Interval between OIDC self-heal watchdog runs. |
| `headscale_selfjoin_user` | `""` | When set, the role ensures this Headscale user exists and resolves its numeric id into `headscale_selfjoin_user_id`, so the host can mint its own pre-auth key and self-join. Empty on standalone control planes. |
| `headscale_metrics_push_url` | `""` | VictoriaMetrics' `/api/v1/import/prometheus` URL for the tailnet collector. Its host must be an IP address, as the collector's sandbox may reach only that. Empty removes the collector. See notes. |
| `headscale_metrics_push_interval` | `1min` | How often the collector pushes. |
| `headscale_metrics_push_user` | `""` | The vmsingle writer the collector signs in to vmauth as, with basic auth. Empty sends no credentials. |
| `headscale_metrics_push_password` | `""` | Secret. That writer's password, set together with the user. |

## Dependencies

None.

## What it does

1. Downloads the pinned `.deb` to `/tmp/{{ headscale_deb_name }}` (`0644`), verified against `headscale_checksum`.
2. Installs it with apt, which creates the `headscale` systemd service and the `headscale` group, then holds the package (`dpkg_selections`) so unattended-upgrades can never change or autoremove it.
3. Renders `/etc/headscale/config.yaml` (`0640`, group `headscale`) from `config.yaml.j2`: `server_url`, listen/metrics/gRPC addresses, IPv4/IPv6 tailnet prefixes, a self-hosted embedded DERP relay (region 999, STUN on `0.0.0.0:3478`, no public Tailscale DERP), a sqlite database, a TLS-ALPN-01 Let's Encrypt cert on 443, MagicDNS, and (only when `headscale_oidc_enabled`) the OIDC block.
4. When `headscale_metrics_nonlocal_bind`, sets `net.ipv4.ip_nonlocal_bind=1` in `/etc/sysctl.d/60-headscale-nonlocal-bind.conf` and on the running kernel, so the metrics address can be bound before it exists on an interface. Otherwise it removes that file and sets the running value to `0`.
5. Probes the listen port and enables the service, restarting it when the config changed, the sysctl changed, or nothing is listening, otherwise just ensuring it is started; then `wait_for`s the port.
6. When `headscale_selfjoin_user` is set, ensures that user exists (tolerating "already exists") and resolves its numeric id into `headscale_selfjoin_user_id`.
7. When `headscale_oidc_enabled`, installs the OIDC self-heal watchdog (`headscale-oidc-watchdog.sh` plus its `.service`/`.timer` units) and enables the timer. Headscale loads OIDC discovery only at startup, so an issuer unreachable then leaves it in CLI-only enrolment; the watchdog restarts Headscale once the issuer is reachable. A one-shot heal runs on install when the script or units changed, so a currently-degraded server recovers without waiting for the first timer interval.
8. With `headscale_metrics_push_url` set, installs the tailnet collector (`/usr/local/sbin/headscale-metrics` and its `.service`/`.timer` units) and enables its timer. With a push user, it writes the password to `/etc/headscale/metrics-push-password` (`0640`, group `headscale`), so it's on no command line; without one, it removes that file. Cleared, it stops the timer and removes them, the password file included.

Handlers: none — the running service is reconciled to the on-disk config directly in tasks (see Notes).

## Example

```yaml
- hosts: overlay_control
  roles:
    - role: headscale
      vars:
        headscale_arch: arm64
        headscale_listen_addr: "127.0.0.1:8443"   # behind sni_router
        headscale_oidc_enabled: true
        headscale_oidc_issuer: "https://auth.example.com/application/o/headscale/"
        headscale_oidc_client_id: "{{ vault_hs_client_id }}"
        headscale_oidc_client_secret: "{{ vault_hs_client_secret }}"
```

## Notes

- `headscale_oidc_only_start_if_available` is false on purpose. The EC2 reaches Authentik over the very overlay Headscale runs, so a hard OIDC-at-startup dependency deadlocks (Authentik blip -> Headscale won't start -> overlay degrades -> can't reach Authentik). Starting anyway breaks the loop; OIDC enrolment resumes once Authentik is reachable.
- `headscale_oidc_scopes` deliberately omits `groups`: that scope triggers Headscale's "empty OIDC callback params" failure (headscale#2887). Authentik's `profile` scope already carries the groups claim that `allowed_groups` matches against.
- `headscale_metrics_listen_addr` is Headscale's debug server, not just `/metrics`. Through tailscale's `tsweb.Debugger` it serves `/debug/config` (the running config as JSON, OIDC client secret included), the policy, routes and pprof, to any loopback or Tailscale source address. On the tailnet IP, every enrolled device can read it unless an ACL policy stops it. Keep it on loopback and scrape it from an agent on the same host.
- The `.deb` is verified against Headscale's published checksums file by default (`headscale_checksum`). This catches transport corruption and tampering of the artifact; for a stronger guarantee against a compromised release, override `headscale_checksum` with a literal `sha256:<hash>` pinned to a reviewed version.
- With OIDC off, bootstrap routers and the laptop with short-lived pre-auth keys issued by hand.
- `headscale_metrics_nonlocal_bind` breaks a boot cycle specific to a host that joins its own overlay: `metrics_listen_addr` is that host's tailnet IP, which only appears once it joins, but Headscale binds the address at startup and would crash-loop on `cannot assign requested address`. Allowing non-local bind lets it claim the address early; nothing can reach it until the overlay is up anyway. It also hides a wrong address: Headscale binds an IP the host doesn't hold without an error. Setting it back to `false` removes the file and turns the setting off host-wide, then restarts Headscale so it runs without it now, not first at the next boot. The role owns this key: don't set `net.ipv4.ip_nonlocal_bind` for anything else on a Headscale host.
- The service is reconciled directly rather than via a notify handler. A handler is deferred to the end of the play, so an interrupted run (or a later role that dials Headscale) would see a service still on its install-time config. Restarting on "config/sysctl changed or port not listening" converges from any state.
- The tailnet collector pushes, as nothing in the lab can reach the control server to scrape it. Every `headscale_metrics_push_interval` it reads `headscale nodes list`, `users list` and each user's `preauthkeys list` as JSON and POSTs Prometheus text with `job=headscale` and `instance=<inventory_hostname>` labels: `headscale_node_online{node,user}`, `headscale_node_last_seen_timestamp_seconds{node,user}`, `headscale_node_route{node,user,route,state}` (state `available`, `approved` or `serving`, the last being the node Headscale made primary), `headscale_preauthkeys_usable` (unexpired, and reusable or never used; Headscale 0.26.1 leaves a reusable key's `used` unset however often it joins) and `headscale_metrics_collected_timestamp_seconds`. The CLI's JSON carries each key's secret; only the count is pushed. The push is built whole before it is sent, so a failed read pushes nothing and the unit fails. It runs as the `headscale` user, for the CLI socket, with no capabilities, and reaches only the push URL's address. The monitoring role's `PushedMetricsStale` alert watches the timestamp. `tests/metrics.yml` runs it against the pinned package under systemd, three real Tailscale nodes and a real vmsingle.
- `headscale_selfjoin_user` supports a host that is its own control server: the role ensures the user and exposes its id, and the `tailscale` role's `tailscale_authkey_command` mints a fresh key against the live database at join time — no perishable key in SOPS.
