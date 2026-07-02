# monitoring

Deploys the lab observability stack (VictoriaMetrics, vmalert, Alertmanager to ntfy, Grafana, plus exporters and a go2rtc CCTV restream) on one Incus host via docker compose.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- `community.docker` collection.
- A container/host the `docker` role can install Docker Engine and Compose v2 onto.
- Secrets (Grafana admin/OIDC, ntfy passwords, SNMPv3 creds, go2rtc RTSP URLs, HASS token) supplied from SOPS.
- An Authentik instance for Grafana OIDC.
- Root via `become`.

## Role variables

### Compose and images

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_compose_dir` | `/opt/monitoring` | Host directory for the compose file, `.env`, and data/config trees. |
| `monitoring_data_dir` | `{{ monitoring_compose_dir }}/data` | Persistent data root. |
| `monitoring_config_dir` | `{{ monitoring_compose_dir }}/config` | Rendered config root (scrape, rules, Grafana provisioning, etc.). |
| `monitoring_vmsingle_image` | `victoriametrics/victoria-metrics:v1.102.1` | VictoriaMetrics single-node (storage + scraping). |
| `monitoring_vmalert_image` | `victoriametrics/vmalert:v1.102.1` | vmalert (rule evaluation). |
| `monitoring_alertmanager_image` | `prom/alertmanager:v0.27.0` | Alertmanager. |
| `monitoring_grafana_image` | `grafana/grafana:11.2.0` | Grafana. |
| `monitoring_ntfy_image` | `binwiederhier/ntfy:v2.11.0` | Self-hosted ntfy. |
| `monitoring_redis_exporter_image` | `oliver006/redis_exporter:v1.62.0` | Redis exporter. |
| `monitoring_blackbox_image` | `prom/blackbox-exporter:v0.25.0` | Blackbox exporter (https probes). |
| `monitoring_snmp_exporter_image` | `prom/snmp-exporter:v0.26.0` | SNMP exporter. |
| `monitoring_go2rtc_image` | `alexxit/go2rtc:1.9.14` | go2rtc CCTV restream. |

### Retention, scrape, and kiosk

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_retention` | `"30d"` | Metric retention. Modest, given constrained storage. |
| `monitoring_scrape_interval` | `"30s"` | Global scrape interval. |
| `monitoring_kiosk_playlist_name` | `"Lab Kiosk"` | Name of the Grafana kiosk playlist created via the API. |
| `monitoring_kiosk_tag` | `"kiosk"` | Dashboards carrying this tag are rotated in the kiosk playlist. |
| `monitoring_kiosk_rotate_interval` | `"30s"` | Playlist rotation interval. |

### Scrape targets (from inventory, no IPs in the role)

Each `*_targets` list holds `{target: "ip:port", instance: "hostname"}` mappings so scraped series carry the real hostname as their `instance` label. An empty list omits that job.

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_node_targets` | `[]` | node_exporter on bare-metal hosts (`:9100`). |
| `monitoring_blocky_targets` | `[]` | blocky `/metrics` (`:4000`). |
| `monitoring_patroni_targets` | `[]` | Patroni REST `/metrics` (`:8008`). |
| `monitoring_etcd_targets` | `[]` | etcd `/metrics` (`:2379`). |
| `monitoring_redis_targets` | `[]` | redis endpoints (`:6379`) via redis_exporter. |
| `monitoring_speedtest_targets` | `[]` | speedtest-tracker native `/prometheus` (`:8080`). |
| `monitoring_blackbox_targets` | `[]` | Plain list of https URLs for cert-expiry probes; the URL becomes the `instance` label. |
| `monitoring_snmp_targets` | `[]` | SNMPv3 targets (the Synology NAS): `{target: "ip:161", instance}`. |
| `monitoring_headscale_target` | `""` | Headscale control-plane `/metrics` over the overlay (`tailnet-ip:9090`). |
| `monitoring_hass_target` | `""` | Home Assistant Prometheus scrape via Caddy (`host:443`). |
| `monitoring_hass_token` | `""` | HASS long-lived token (SOPS); injected via container env and expanded by vmsingle, never written to the on-disk scrape config. |

### Alert thresholds

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_temp_warn_celsius` | `75` | CPU temperature warning threshold. |
| `monitoring_disk_used_percent` | `85` | Disk-used warning threshold. |

### Grafana and OIDC

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_grafana_root_url` | `""` | Public Grafana URL (e.g. `https://grafana.example.com`). |
| `monitoring_grafana_admin_user` | `"admin"` | Grafana admin username. |
| `monitoring_grafana_oidc_auth_url` | `""` | Authentik global authorize endpoint. |
| `monitoring_grafana_oidc_token_url` | `""` | Authentik global token endpoint. |
| `monitoring_grafana_oidc_api_url` | `""` | Authentik global userinfo endpoint. |
| `monitoring_grafana_oidc_signout_url` | `""` | Authentik end-session endpoint. |
| `monitoring_grafana_role_path` | `contains(groups[*], 'grafana-admins') && 'Admin' \|\| 'Editor'` | JMESPath mapping Authentik groups to Grafana roles (`grafana-admins` to Admin, otherwise Editor). |
| `monitoring_grafana_admin_password` | `""` | SOPS. |
| `monitoring_grafana_oidc_client_id` | `""` | SOPS. |
| `monitoring_grafana_oidc_client_secret` | `""` | SOPS. |

### ntfy

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_ntfy_base_url` | `""` | Public base URL ntfy serves on. |
| `monitoring_ntfy_topic` | `"lab-alerts"` | Alert topic. |
| `monitoring_ntfy_user` | `""` | Human username (web UI + phone app), from the deploy. |
| `monitoring_ntfy_alertmanager_password` | `""` | SOPS — the alertmanager publisher user. |
| `monitoring_ntfy_user_password` | `""` | SOPS — the human subscriber user. |
| `monitoring_redis_password` | `""` | SOPS — redis_exporter connection password. |

### SNMPv3 (Synology NAS, authPriv)

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_snmp_v3_username` | `""` | SOPS. |
| `monitoring_snmp_v3_auth_password` | `""` | SOPS — SNMPv3 auth password. |
| `monitoring_snmp_v3_priv_password` | `""` | SOPS — SNMPv3 privacy (encryption) password. |
| `monitoring_snmp_v3_auth_protocol` | `"SHA"` | Auth protocol (matched on the DSM side). |
| `monitoring_snmp_v3_priv_protocol` | `"AES"` | Privacy protocol (matched on the DSM side). |

### go2rtc CCTV restream

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_go2rtc_api_port` | `1984` | go2rtc HTTP port (MSE/HLS/UI). |
| `monitoring_go2rtc_front_url` | `""` | SOPS — front camera RTSP source (carries a Scrypted access token). |
| `monitoring_go2rtc_garage_carport_url` | `""` | SOPS — garage/carport camera RTSP source (carries a Scrypted access token). |
| `monitoring_go2rtc_username` | `""` | Optional go2rtc Basic auth username. Empty leaves auth off. |
| `monitoring_go2rtc_password` | `""` | Optional go2rtc Basic auth password. |
| `monitoring_go2rtc_public_url` | `""` | Browser-facing go2rtc base URL (Caddy front) the Grafana CCTV iframe points at. |

## Dependencies

- `docker` role (meta dependency) — installs Docker Engine and Compose v2.

## What it does

Creates the compose, config, and Grafana provisioning directories, then renders every config file: the scrape config, alert rules, Alertmanager config, ntfy config, blackbox and snmp exporter configs, the go2rtc config, Grafana datasource and dashboard provisioning, and the dashboard JSON. Secrets (the `.env`, and the HASS token file at mode `0600`) are rendered separately. It then brings the stack up with `docker_compose_v2`.

After the stack is up it runs two API-driven, idempotent provisioning steps:

- ntfy starts deny-all, so the role waits for it, then creates the `alertmanager` (write-only) and human subscriber (read-only) users and sets their topic ACLs. Re-adding an existing user is tolerated.
- The Grafana file provisioner can't create playlists, so the role waits for Grafana, checks whether the kiosk playlist exists, and creates it via the API if not. The playlist rotates every dashboard tagged `monitoring_kiosk_tag`, so new dashboards join just by carrying the tag.

vmsingle is used instead of Prometheus for lighter storage and better compaction on the resource- and heat-constrained rig; it is PromQL-compatible, so Grafana treats it as a Prometheus datasource. Any config, env, or compose change restarts the stack.

## Example

```yaml
- hosts: monitoring_host
  roles:
    - role: monitoring
      vars:
        monitoring_grafana_root_url: "https://grafana.example.com"
        monitoring_grafana_admin_password: "{{ vault_grafana_admin_password }}"
        monitoring_grafana_oidc_client_id: "{{ vault_grafana_oidc_client_id }}"
        monitoring_grafana_oidc_client_secret: "{{ vault_grafana_oidc_client_secret }}"
        monitoring_ntfy_base_url: "https://ntfy.example.com"
        monitoring_node_targets:
          - { target: "10.0.30.11:9100", instance: "node-a" }
          - { target: "10.0.30.12:9100", instance: "node-b" }
        monitoring_go2rtc_front_url: "{{ vault_go2rtc_front_url }}"
        monitoring_go2rtc_public_url: "https://cctv.example.com"
```

## Notes

Grafana OIDC uses Authentik's global (not per-application) endpoints. Group-to-role mapping is done through `monitoring_grafana_role_path`.

The go2rtc RTSP source URLs and the HASS scrape token are secrets that carry access tokens. They are injected via the container environment and expanded at config-load time, so they are never written in plaintext to the on-disk config. go2rtc publishes only its HTTP port; WebRTC (UDP/TCP 8555) is left for on-LAN use and is not published here. Enabling go2rtc Basic auth would make the browser prompt for credentials inside the Grafana iframe and break the embed; the auth boundary is Caddy TLS, lab-only reachability, and Grafana's Authentik OIDC.
