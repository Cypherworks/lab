# sni_router

L4 SNI passthrough router (nginx `stream` + `ssl_preread`) that proxies raw TLS connections by SNI without terminating TLS.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu on the target (apt, systemd). Installs `nginx` and `libnginx-mod-stream`.
- Ansible `ansible.builtin` only. No external collections.
- Backends that terminate their own TLS (Headscale locally, or the lab Caddy over the overlay).

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `sni_listen_port` | `443` | Port the stream server listens on. |
| `sni_routes` | `[]` | Site data. List of `{ sni: <hostname>, upstream: <host:port> }` route entries. |
| `sni_default_upstream` | `127.0.0.1:8443` | Backend for an unrecognised SNI (Headscale, which rejects it). |
| `sni_proxy_timeout` | `10m` | Stream proxy idle timeout. Long, because Headscale holds long-poll connections. |

`sni_routes` is empty by default and supplied by the deployment.

## Dependencies

None.

## What it does

1. Installs `nginx` and `libnginx-mod-stream`.
2. Renders `/etc/nginx/nginx.conf` (`0644`) from `nginx.conf.j2`, validated with `nginx -t -c %s` before it is written. The config is stream-only: a `map $ssl_preread_server_name $sni_upstream` built from `sni_routes` (with `default` pointing at `sni_default_upstream`), and one `server` that listens on `sni_listen_port` with `ssl_preread on`, `proxy_pass $sni_upstream` and `proxy_timeout {{ sni_proxy_timeout }}`.
3. Removes `/etc/nginx/sites-enabled/default` (this box serves no HTTP).
4. Probes `sni_listen_port` and enables nginx, restarting it when the config changed or nothing is listening on that port, otherwise just ensuring it is started.
5. `wait_for`s the port so a bring-up failure lands here rather than on a downstream role that dials through the router.

Handlers: none — the running service is reconciled to the on-disk config directly in tasks (see Notes).

## Example

```yaml
- hosts: edge
  roles:
    - role: sni_router
      vars:
        sni_routes:
          - { sni: "headscale.example.com", upstream: "127.0.0.1:8443" }
          - { sni: "auth.example.com",      upstream: "10.0.30.10:443" }
```

## Notes

- No TLS is terminated here. `ssl_preread` reads the SNI from the ClientHello and the raw connection is proxied on untouched, so the full handshake (including the `acme-tls/1` ALPN that Headscale's TLS-ALPN-01 renewal needs) happens end to end between client and backend.
- The point is to keep a public host (the EC2) a dumb pipe: it routes by SNI but cannot read the auth login or any other plaintext.
- `nginx -t` gates the config write, so a bad map or upstream fails the task rather than reloading nginx into a broken state.
- The service is reconciled directly rather than via a notify handler. A handler is deferred to the end of the play, so a run that stops early (or a later role in the same play that dials through this router) would see stale nginx. Restarting on "config changed or port not listening" converges from any state, including a box left half-configured by an interrupted run, and a restart (not reload) ensures the `stream` module is loaded.
