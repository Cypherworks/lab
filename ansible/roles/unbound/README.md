# unbound

Recursive, DNSSEC-validating resolver listening on loopback (`127.0.0.1:5335` by
default). The DNS frontend (AdGuard Home) forwards to it, so the lab resolves
from the root rather than trusting a public resolver. Not exposed to the network.

## Vars (defaults in `defaults/main.yml`)
- `unbound_listen` / `unbound_port` — loopback bind.
- `unbound_cache_min_ttl` / `_max_ttl`, `unbound_edns_buffer_size`.

Config is validated with `unbound-checkconf` before reload.
