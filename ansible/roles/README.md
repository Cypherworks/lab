# Ansible roles

Generic, parameterised roles. Each is a mechanism: it takes behaviour from variables and defaults and expects site data (addresses, hostnames, secrets) from the consuming inventory and SOPS. Each role has its own README with variables, dependencies, and an example.

The grouped index with one-line descriptions is in the [repository README](../../README.md#ansible-roles).

| Role | Purpose |
|------|---------|
| `base` | Baseline OS configuration for every host. |
| `docker` | Docker Engine and Compose v2. Shared dependency. |
| `unattended_upgrades` | Automatic security updates with a quiet-window reboot. |
| `ssh_ca_trust` | Trust an SSH CA for user certificates via an additive sshd drop-in. |
| `sssd` | SSSD LDAP client against an Authentik LDAP outpost. |
| `tailscale` | Join a host to a Headscale/Tailscale overlay. |
| `blocky` | DNS frontend with blocklists, forwarding to a local recursive resolver. |
| `unbound` | Recursive validating resolver on loopback. |
| `keepalived` | VRRP floating IP with a DNS-query health check. |
| `caddy` | Caddy reverse-proxy configuration and trusted internal CAs. |
| `sni_router` | L4 SNI passthrough router (nginx stream). |
| `headscale` | Headscale control server with optional OIDC. |
| `etcd` | etcd cluster serving as the Patroni configuration store. |
| `postgres_patroni` | PostgreSQL under Patroni with automatic failover. |
| `haproxy_patroni` | Node-local HAProxy routing to the Patroni primary and Redis master. |
| `redis_sentinel` | Redis with Sentinel for automatic master promotion. |
| `incus` | Incus with web UI, per-VLAN bridges, storage, and optional clustering. |
| `openbao` | OpenBao secrets manager — PKI, KMS auto-unseal, OIDC, SSH CA, snapshots. |
| `authentik` | Authentik identity provider, standalone deployment. |
| `authentik_app` | Authentik app tier against an external HA database, with an LDAP outpost. |
| `monitoring` | VictoriaMetrics, vmalert, Alertmanager, ntfy, Grafana, and exporters. |
| `node_exporter` | Prometheus node_exporter on bare-metal hosts. |
| `unifi_controller` | Self-hosted UniFi Network controller and MongoDB. |
| `vaultwarden` | Vaultwarden password manager. |
| `speedtest` | Speedtest Tracker with a Prometheus endpoint. |
| `rpi_poe_fan` | Quiet PoE HAT fan thresholds. |
| `rpi_radios` | Disable onboard WiFi and Bluetooth at firmware level. |
