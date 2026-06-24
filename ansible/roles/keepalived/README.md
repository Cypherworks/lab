# keepalived

VRRP floating IP for an active/standby service pair (the DNS VIP here). The
active node holds the VIP; a `vrrp_script` health check drops its priority if the
service is down, moving the VIP to the peer.

## Vars (defaults in `defaults/main.yml`)
- `keepalived_vip` — the shared VIP (required).
- `keepalived_interface`, `keepalived_vrid`.
- Per-host (host_vars): `keepalived_state` (MASTER/BACKUP) + `keepalived_priority`.
- `keepalived_auth_pass` — shared VRRP secret (from SOPS).
- `keepalived_check_command` / `keepalived_check_weight` — health check.

Service binds need to answer on the VIP; the DNS frontend binds `0.0.0.0`, so it
serves the VIP whenever keepalived assigns it (no `ip_nonlocal_bind` needed).
