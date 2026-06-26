# sni_router

An L4 SNI passthrough router (nginx `stream` + `ssl_preread`) for the Headscale
EC2. It listens on :443, reads the SNI from the TLS ClientHello **without
terminating TLS**, and proxies the raw connection to a backend chosen by hostname.

This lets one public :443 serve two names:

- `headscale.cypherworks.co.uk` → Headscale on a local port (`127.0.0.1:8443`).
  Raw passthrough preserves the `acme-tls/1` ALPN, so Headscale's TLS-ALPN-01 cert
  renewal and its embedded DERP relay keep working behind the router.
- `auth.cypherworks.co.uk` → the lab Caddy (`10.200.30.10:443`) over the Tailscale
  overlay (the EC2 must be on the tailnet — see the `tailscale` role). Caddy
  terminates with the wildcard cert and proxies to Authentik.

Because it never terminates the `auth.cw` TLS, a compromise of the EC2 cannot read
or forge the Authentik login — it's a dumb encrypted pipe.

| Variable | Purpose |
| --- | --- |
| `sni_routes` | `[{ sni, upstream }]` host→backend map |
| `sni_default_upstream` | where an unknown SNI goes |
| `sni_listen_port` | public listen port (443) |
