# monitoring

Deploys the lab observability stack (VictoriaMetrics, VictoriaLogs, vmalert, Alertmanager to Discord and healthchecks.io, Grafana, plus exporters and a go2rtc CCTV restream) on one Incus host via docker compose.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- `community.docker` collection.
- A container/host the `docker` role can install Docker Engine and Compose v2 onto.
- Secrets (Grafana admin/OIDC, Discord and healthchecks.io URLs, SNMPv3 creds, go2rtc RTSP URLs, HASS token) supplied from SOPS.
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
| `monitoring_vmauth_image` | `victoriametrics/vmauth:v1.102.1` | vmauth, the only way in to vmsingle and VictoriaLogs from outside the stack. Keep it on vmsingle's version. |
| `monitoring_alertmanager_image` | `prom/alertmanager:v0.34.2` | Alertmanager. `webhook_url_file` needs 0.28 or later. |
| `monitoring_grafana_image` | `grafana/grafana:13.3.0` | Grafana. 13.0 moves dashboards into unified storage with a one-way migration, so roll back from a snapshot, not by pinning an older image. Grafana runs only the plugins the image ships: startup plugin installs are off and the plugins directory is an empty read-only mount. |
| `monitoring_redis_exporter_image` | `oliver006/redis_exporter:v1.62.0` | Redis exporter. |
| `monitoring_blackbox_image` | `prom/blackbox-exporter:v0.25.0` | Blackbox exporter (https probes). |
| `monitoring_snmp_exporter_image` | `prom/snmp-exporter:v0.26.0` | SNMP exporter. |
| `monitoring_go2rtc_image` | `alexxit/go2rtc:1.9.14` | go2rtc CCTV restream. |
| `monitoring_victorialogs_image` | `victoriametrics/victoria-logs:v1.51.0` | VictoriaLogs (log storage + query). Not a Grafana datasource: Viewer can query any datasource, and anonymous Viewer can be on. Read it in its own UI at `/select/vmui/`, behind your proxy's login, as a VictoriaLogs reader. |

### Retention and scrape

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_retention` | `"30d"` | Metric retention. Modest, given constrained storage. |
| `monitoring_logs_retention` | `"30d"` | VictoriaLogs retention, on the same constrained budget. |
| `monitoring_victorialogs_syslog_port` | `null` | Host UDP port published to VictoriaLogs' syslog listener, for devices with no log shipper. Lines are stamped on receipt and carry the sender as `remote_ip`; fields are `app_name`, `hostname`, `level` and `_msg`, with `{app_name, hostname}` as the stream. Unauthenticated and cleartext, like the HTTP ingest on 9428. `null` leaves the listener off. |
| `monitoring_scrape_interval` | `"30s"` | Global scrape interval. |
| `monitoring_grafana_anonymous_enabled` | `false` | Anonymous view-only Grafana access (for the wall kiosk); admin/edit stays behind Authentik OIDC. |

### vmsingle access

vmsingle is never published. vmauth answers on 8428 in its place, and only when there are readers or writers; with neither, vmsingle is reachable only on the compose network. vmalert and Grafana stay on the compose network and talk to vmsingle directly.

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_vmsingle_readers` | `[]` | Secrets. `{name, password}` users who may run instant and range queries (`/api/v1/query`, `/api/v1/query_range`) and nothing else. |
| `monitoring_vmsingle_writers` | `[]` | Secrets. `{name, password, job, instance}` users who may import Prometheus text (`/api/v1/import/prometheus`) and nothing else. vmauth pins `job` and `instance` on every series a writer sends and drops any `extra_label` it asks for, so a writer's password can't write under another source's name. |
| `monitoring_vmsingle_admin_key` | `""` | Secret, required. The key vmsingle's admin endpoints (delete, snapshots, force merge and flush, cache reset, config, reload, flags, pprof, metrics) and vmauth's own (metrics, flags, pprof, reload) want as `authKey`. vmauth routes no admin path to anyone, so only something on the compose network can use it. |

### VictoriaLogs access

VictoriaLogs' HTTP port is never published either. The same vmauth answers on 9428 in its place, only when there are VictoriaLogs readers or writers. vmalert-logs reads it on the compose network directly, and vmsingle scrapes its metrics there. The syslog port, when `monitoring_victorialogs_syslog_port` is set, is published as it is: UDP syslog can't sign in, so anything that reaches it can write lines, under any `hostname`.

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_victorialogs_readers` | `[]` | Secrets. `{name, password}` users who may use `/select/`: LogsQL queries and the UI at `/select/vmui/`. They can't ingest or reach the admin endpoints. |
| `monitoring_victorialogs_writers` | `[]` | Secrets. `{name, password, host}` users who may send Elasticsearch bulk lines (`/insert/elasticsearch/_bulk`, as the vector role ships) and nothing else. vmauth sets `host` on every line a writer sends, over whatever host the line, an `extra_fields` argument or a `VL-Extra-Fields` header names, so a writer's password can't write under another host's name. `host` is letters, digits, dots, dashes and underscores. |
| `monitoring_victorialogs_admin_key` | `""` | Secret, required. The key VictoriaLogs' partition, force merge and flush, new-stream logging, flags and pprof endpoints want as `authKey`; without it anything that reached the port could detach a day's partition and hide its logs. Its `/metrics` stays open for vmsingle's scrape. `/delete/*` is off, as the role never passes `-delete.enable`. |

Names are unique across all four lists, and a password can't hold `%{`, which vmauth reads as an environment placeholder. vmauth compares passwords in plain text, so its config is a root-only `0600` file.

### Scrape targets (from inventory, no IPs in the role)

Each `*_targets` list holds `{target: "ip:port", instance: "hostname"}` mappings so scraped series carry the real hostname as their `instance` label. An empty list omits that job. The stack's own metrics are always scraped, as jobs `alertmanager`, `vmalert` and `victorialogs`, so `TargetDown` covers them too.

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_node_targets` | `[]` | node_exporter on bare-metal hosts (`:9100`). |
| `monitoring_node_instance_targets` | `[]` | node_exporter on the Incus instances (`:9100`), scraped as job `node-instance` so a host and an instance stay apart. |
| `monitoring_blocky_targets` | `[]` | blocky `/metrics` (`:4000`). |
| `monitoring_patroni_targets` | `[]` | Patroni REST `/metrics` (`:8008`). |
| `monitoring_etcd_targets` | `[]` | etcd `/metrics` (`:2379`). |
| `monitoring_redis_targets` | `[]` | redis endpoints (`:6379`) via redis_exporter. |
| `monitoring_speedtest_targets` | `[]` | speedtest-tracker native `/prometheus` (`:8080`). |
| `monitoring_blackbox_targets` | `[]` | Plain list of https URLs for cert-expiry probes; the URL becomes the `instance` label. |
| `monitoring_blackbox_icmp_targets` | `[]` | Addresses pinged once per scrape. Scraped as job `icmp`; a failed probe is one lost packet. |
| `monitoring_blackbox_service_targets` | `[]` | Health URLs for the kiosk's up/down tiles: `{url, instance}`. Scraped as job `service-health`; its cert series are dropped. An `https://` URL must answer over TLS; an `http://` one (inside the lab) is probed with the `http_2xx` module instead. `ServiceDown` fires when one fails for a minute. |
| `monitoring_blackbox_cctv_targets` | `[]` | One frame URL per camera (go2rtc's `/api/frame.jpeg?src=<stream>`), as `{url, instance}`. Scraped as job `cctv` with the `http_jpeg` module, which passes only on a JPEG; its cert series are dropped. |
| `monitoring_blackbox_cctv_interval` | `"60s"` | How often job `cctv` runs. Each probe decodes a frame. |
| `monitoring_blackbox_node_tls_targets` | `[]` | HA nodes to read a cert from directly, past their VIP: `{target: "ip:443", instance}`. Scraped as job `blackbox-tls`. |
| `monitoring_blackbox_node_tls_server_name` | `""` | SNI name those nodes are asked for. |
| `monitoring_blackbox_tls_skip_verify` | `false` | Skip verification on https probes. A cert that fails verification reports no expiry at all, so set this to see an expired cert. |
| `monitoring_cert_expiry_warn_days` | `14` | `CertExpiringSoon` fires under this many days left. `CertNotProbed` fires when fewer targets report a cert than are configured. |
| `monitoring_blackbox_dns_targets` | `[]` | DNS resolvers to probe, as `{target: "ip:53", instance}`. Scraped as job `dns`; `probe_success` is 1 when one answers, and `ServiceDown` fires when one fails for a minute. |
| `monitoring_blackbox_dns_query` | `""` | The name each resolver is asked. Choose one it answers itself, so the check survives an upstream outage. |
| `monitoring_snmp_targets` | `[]` | SNMPv3 targets (the Synology NAS): `{target: "ip:161", instance}`. |
| `monitoring_incus_metrics_targets` | `[]` | Incus cluster `/1.0/metrics` over mTLS: `{target: "member-ip:8444", instance}`. Empty omits the job. |
| `monitoring_incus_instances` | `[]` | Instance names that should be running. `InstanceNotRunning` fires when Incus reports nothing for one for a minute, as it drops every series of a stopped instance. Empty leaves the rule out. |
| `monitoring_pushed_metrics` | `[]` | Sources that push to vmsingle instead of being scraped, as `{name, metric}` with `metric` the source's push timestamp gauge. `PushedMetricsStale` fires for one whose last push is older than `monitoring_pushed_metrics_stale_seconds`, or that has never pushed, for two minutes. Empty leaves the rule out. |
| `monitoring_pushed_metrics_stale_seconds` | `300` | How old a pushed source's last push may be before `PushedMetricsStale` counts it stopped. |
| `monitoring_vmalert_logs_image` | `victoriametrics/vmalert:v1.154.0` | vmalert-logs' image. LogsQL rules need v1.106.0 or later; the main vmalert stays on `monitoring_vmalert_image`. |
| `monitoring_log_rules` | `[]` | Recording rules over VictoriaLogs, as `{record, expr}` with `expr` LogsQL ending in `stats`. vmalert-logs, a second vmalert since one takes a single datasource, evaluates them and writes the counts to vmsingle; `stats by (field)` fields become labels. Empty leaves vmalert-logs out of the compose. |
| `monitoring_log_rules_interval` | `1m` | How often vmalert-logs evaluates; vmalert appends `_time:<interval>`, so each sample counts that much of the log, and `sum_over_time` adds them up. |
| `monitoring_not_logging_metric` | `""` | A `monitoring_log_rules` record counted `by (host)`. `NotLogging` fires for a system in `monitoring_not_logging_jobs` that is up but has no sample in `monitoring_not_logging_window`, for five minutes; a down system is left to `TargetDown`, and an absent record fires for every system. Empty leaves the rule out. |
| `monitoring_not_logging_jobs` | `[]` | Scrape jobs whose instances must log. |
| `monitoring_not_logging_window` | `30m` | How long a system may go without a log line before `NotLogging` counts it silent. |
| `monitoring_alert_rules` | `[]` | The site's own vmalert alerting rules, each `{alert, expr, for, labels, annotations}` as vmalert reads them, rendered after the role's rules in the same group. Ansible templates the strings, so escape any alert templating (`description: "At {{ '{{ $value }}' }}"`). `tests/site_alert_rules.yml` runs one through promtool and vmalert. |
| `monitoring_incus_metrics_cert` | `""` | Metrics scraper client cert (PEM); the TF-generated keypair, mounted as a docker secret. |
| `monitoring_incus_metrics_key` | `""` | Metrics scraper client key (PEM); mounted as a docker secret. |
| `monitoring_headscale_target` | `""` | Headscale control-plane `/metrics` over the overlay (`tailnet-ip:9090`). |
| `monitoring_hass_target` | `""` | Home Assistant Prometheus scrape via Caddy (`host:443`). |
| `monitoring_hass_token` | `""` | HASS long-lived token (SOPS); rendered to a `0600` file mounted into vmsingle as a docker secret and read via `credentials_file`, never written into the scrape config. |

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

### Alert receivers

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_alertmanager_repeat_interval` | `"24h"` | How often Alertmanager re-notifies a still-firing alert. |
| `monitoring_discord_critical_webhook_url` / `monitoring_discord_warning_webhook_url` | `""` | Secrets. Discord webhooks by severity, written to files. Text sent there goes through `alert-text.tmpl`, which strips URLs, FQDNs and addresses. Empty leaves Discord out. |
| `monitoring_healthchecks_ping_url` | `""` | Secret. Set, a `Watchdog` alert pings it every minute, so an outside service can notice the lab has gone. A period of 2 minutes with 1 of grace alerts about 3 minutes after the last ping. |
| `monitoring_redis_password` | `""` | SOPS — redis_exporter connection password. |
| `monitoring_alertmanager_api_users` | `[]` | Secrets. Readers of the Alertmanager API from outside the instance, as `{name, password}`. Set, Alertmanager is published on 9093 behind basic auth (`--web.config.file`). Empty keeps it on the compose network only. |
| `monitoring_alertmanager_vmalert_password` | `""` | Secret, needed with API users. vmalert signs in to Alertmanager as `vmalert` with it; that name is reserved. |
| `monitoring_alertmanager_vmsingle_password` | `""` | Secret, needed with API users. vmsingle signs in to Alertmanager as `vmsingle` with it to scrape its metrics; that name is reserved. |

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
| `monitoring_go2rtc_video_transcode` | `false` | Transcode camera video to H.264, for H.265 cameras or sources with no parameter sets in their SDP, which MSE draws at 4:3. Software transcode, costs CPU on the host. |
| `monitoring_go2rtc_video_fps` | `0` | Frame rate limit on the transcode; `0` keeps the camera's rate. A limit also puts keyframes 2 seconds apart: go2rtc's MSE player keeps only 5 seconds and drops a whole keyframe group at a time, so its default 50-frame group (5 seconds at 10 fps) made the playing picture vanish at every keyframe. |
| `monitoring_go2rtc_front_video_filter` | `""` | ffmpeg `-vf` filter chain for the front camera, e.g. `drawbox` to dim the camera's clock or `delogo` to hide its name. Only applied when transcoding. go2rtc splits its stream options on `#` and the command on spaces, so the chain can't hold either: write colours as `0x110d0c`. |
| `monitoring_go2rtc_garage_carport_video_filter` | `""` | The same for the garage/carport camera. |
| `monitoring_go2rtc_front_url` | `""` | SOPS — front camera RTSP source (carries a Scrypted access token). |
| `monitoring_go2rtc_garage_carport_url` | `""` | SOPS — garage/carport camera RTSP source (carries a Scrypted access token). |
| `monitoring_go2rtc_username` | `""` | Optional go2rtc Basic auth username. Empty leaves auth off. |
| `monitoring_go2rtc_password` | `""` | Optional go2rtc Basic auth password. |
| `monitoring_go2rtc_public_url` | `""` | Browser-facing go2rtc base URL (Caddy front) the Grafana CCTV iframe points at. |

### Power sensors (Home Assistant)

| Variable | Default | Description |
|----------|---------|-------------|
| `monitoring_power_sensors` | *map of nine `""` keys* | HASS power/energy sensor entity IDs for the kiosk's power tiles (now W plus day/month kWh, per location: rack, desk, office). Site-specific; empty leaves those blocks with no data. |

## Dependencies

- `docker` role (meta dependency) — installs Docker Engine and Compose v2.

## What it does

Creates the compose, config, and Grafana provisioning directories, then renders every config file: the scrape config, alert rules, Alertmanager config and its text template, blackbox and snmp exporter configs, the go2rtc config, Grafana datasource and dashboard provisioning, and the `lab-kiosk` dashboard laid out for a 1080p wall screen. It then removes dropped provisioned dashboards (`cctv.json`, `lab-services-nas.json`, `lab-overview.json`) so Grafana stops serving them, and the dropped ntfy config files. Secrets (the `.env`, the HASS token file, and — when Incus metrics targets are set — the metrics client cert/key, all at mode `0600` docker-secret sources) are rendered separately. With API users set, it hashes each password on the host with `htpasswd` (bcrypt, read from stdin), rehashing only one that changed, drops users the deploy removed, and writes Alertmanager's web config and the password files vmalert and vmsingle sign in with; with none, it removes those files. It writes the vmsingle and VictoriaLogs admin keys (`0400`) and, with readers or writers set, vmauth's users (`0600`), removing those when there are none. It then brings the stack up with `docker_compose_v2`, removing the container of any service the compose doesn't define, and deletes the dropped ntfy volumes, which held its user database.

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
        monitoring_node_targets:
          - { target: "10.0.30.11:9100", instance: "node-a" }
          - { target: "10.0.30.12:9100", instance: "node-b" }
        monitoring_go2rtc_front_url: "{{ vault_go2rtc_front_url }}"
        monitoring_go2rtc_public_url: "https://cctv.example.com"
```

## Notes

vmauth uses basic auth over plain HTTP, as Alertmanager's API does, so a password crosses the network in the clear. Containers on the compose network can still query and write vmsingle without signing in (not its admin endpoints): vmauth v1.102.1 couldn't be made to sign in to a vmsingle that demands basic auth. The same goes for VictoriaLogs: containers on the compose network can query and ingest without signing in, though not reach its admin endpoints. Grafana's anonymous Viewer, when on, can query every metric through Grafana. `tests/vmsingle_auth.yml` and `tests/victorialogs_auth.yml` check who gets in where, on real vmsingle, VictoriaLogs, vmauth and vmalert.

Grafana OIDC uses Authentik's global (not per-application) endpoints. Group-to-role mapping is done through `monitoring_grafana_role_path`.

The go2rtc RTSP source URLs and the HASS scrape token are secrets that carry access tokens. The RTSP URLs are injected via the container environment and expanded at config-load time; the HASS token is a `0600` docker secret that the scrape job reads via `credentials_file`. Neither is written in plaintext into the on-disk config. go2rtc publishes only its HTTP port; WebRTC (UDP/TCP 8555) is left for on-LAN use and is not published here. Enabling go2rtc Basic auth would make the browser prompt for credentials inside the Grafana iframe and break the embed; the auth boundary is Caddy TLS, lab-only reachability, and Grafana's Authentik OIDC.
