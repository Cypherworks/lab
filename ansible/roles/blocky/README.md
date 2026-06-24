# blocky

DNS frontend: blocklists + allowlists + local DNS records, forwarding to the
local recursive resolver (`unbound`). Config-file-native, so every node gets an
identical `config.yml` from inventory data — no UI, no drift. Exposes Prometheus
metrics on `:4000/metrics`.

Binds `:53`, so the role disables systemd-resolved's stub listener and repoints
`/etc/resolv.conf` at the real upstreams.

## Key vars (defaults in `defaults/main.yml`)
- `blocky_version` / `blocky_arch` — pinned release (arm64 on the Pis).
- `blocky_upstreams` / `blocky_bootstrap` — recursive resolver behind blocky.
- `blocky_denylists` / `blocky_allowlists` — `group -> [urls/inline]`.
- `blocky_client_groups_block` — which denylist groups apply.
- `blocky_custom_dns` — `fqdn -> ip` local records.
- `blocky_block_type`, `blocky_block_ttl`, caching, `blocky_log_level`.

Runs as a non-root user with `CAP_NET_BIND_SERVICE`.
