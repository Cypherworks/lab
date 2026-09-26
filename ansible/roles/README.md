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
| `oob_tunnel` | Break-glass out-of-band access over a persistent outbound reverse SSH tunnel. |
| `blocky` | DNS frontend with blocklists, forwarding to a local recursive resolver. |
| `unbound` | Recursive validating resolver on loopback. |
| `keepalived` | VRRP floating VIP for an active/passive pair, with an optional service health check. |
| `caddy` | Caddy reverse-proxy configuration and trusted internal CAs. |
| `sni_router` | L4 SNI passthrough router (nginx stream). |
| `headscale` | Headscale control server with optional OIDC. |
| `etcd` | etcd cluster serving as the Patroni configuration store. |
| `postgres_patroni` | PostgreSQL under Patroni with automatic failover. |
| `haproxy_patroni` | Node-local HAProxy routing to the Patroni primary and Redis master. |
| `redis_sentinel` | Redis with Sentinel for automatic master promotion. |
| `incus` | Incus (CLI and API, no web UI) with per-VLAN bridges, storage, and optional clustering. |
| `proxmox` | Standalone Proxmox VE host (PVE 9.x / Debian 13) on a stock install. |
| `esxi` | Standalone ESXi 8 host on a stock install. |
| `vcsa` | vCenter Server Appliance deployed onto a standalone ESXi host, with a CA-signed machine SSL certificate. |
| `vcenter_cluster` | vCenter Datacenter and Cluster (HA/DRS/vSAN off) with the ESXi host added. |
| `vcenter_oidc` | vCenter identity provider federation against an external OIDC provider. |
| `openbao` | OpenBao secrets manager — PKI, KMS auto-unseal, OIDC, SSH CA, snapshots. |
| `authentik_app` | Authentik app tier against an external HA database, with an LDAP outpost. |
| `monitoring` | VictoriaMetrics, vmalert, Alertmanager, ntfy, Grafana, VictoriaLogs, and exporters. |
| `node_exporter` | Prometheus node_exporter on bare-metal hosts. |
| `vector` | Per-host log shipping of the systemd journal to VictoriaLogs. |
| `unifi_controller` | Self-hosted UniFi Network controller and MongoDB. |
| `vaultwarden` | Vaultwarden password manager. |
| `speedtest` | Speedtest Tracker with a Prometheus endpoint. |
| `kiosk` | Fullscreen Chromium kiosk (X11) on a headless Ubuntu host, as a systemd service. |
| `wol` | Wake-on-LAN web app for powering on burst hosts. |
| `claude` | AI-assisted development workbench: CI toolchain, Claude Code, and the cw-claude GitHub App helper scripts. |
| `github_runner` | Self-hosted, ephemeral GitHub Actions runner as a container on a Docker host. |
| `infra_runner` | x86_64 Linux control host an operator runs the estate's Terraform and Ansible from. |
| `s3_backup` | Scheduled backup to S3 via a systemd oneshot and timer. |
| `rpi_poe_fan` | Quiet PoE HAT fan thresholds. |
| `rpi_radios` | Disable onboard WiFi and Bluetooth at firmware level. |
