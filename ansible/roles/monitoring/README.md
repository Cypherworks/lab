# monitoring

The observability stack on one Incus instance (docker compose):

- **VictoriaMetrics (`vmsingle`)** — metrics storage + scraping (PromQL, lighter
  than Prometheus, better compaction). Scrape targets come from the deploy.
- **vmalert** — evaluates the alert rules against vmsingle.
- **Alertmanager → alertmanager-ntfy → ntfy** — alerts pushed to your phone via a
  self-hosted ntfy (no third party). *The bridge config/image is unverified — see
  the inline NOTE in `am-ntfy.yml.j2` and confirm on apply.*
- **Grafana** — VictoriaMetrics datasource + a provisioned `Lab Overview`
  dashboard, SSO via Authentik (the `grafana` blueprint + `grafana-users` group).
- **blackbox_exporter** — https probes for TLS cert expiry
  (`probe_ssl_earliest_cert_expiry`). Targets: `monitoring_blackbox_targets`
  (https URLs). Module `https_2xx`.
- **snmp_exporter** — the Synology NAS over **SNMPv3 (authPriv, SHA+AES)**. A
  hand-written `snmp.yml` walks only the OIDs we need (system status/temp, per-disk
  temp/status, RAID volume free/total). Target: `monitoring_snmp_targets`. The v3
  auth/priv passwords come from the container env (`.env`) and are expanded at load
  time (`--config.expand-environment-variables`), so no plaintext lands on disk.

Scrape targets are `{target: "ip:port", instance: "hostname"}` mappings supplied
by the deploy, so series carry the real hostname as their `instance` label
(dashboards read hostnames, not IPs). No per-node node_exporter change. Headscale
is scraped over the overlay via `monitoring_headscale_target` (its tailnet IP:9090).

Pairs with the `node_exporter` host role (CPU/mem/disk/**temperature**). Day-one
alerts: high CPU temp, target down, disk > threshold, Patroni no-primary, Redis
down — all thresholds tunable from the deploy.

Grant a user Grafana by adding them to **grafana-users** (Admin via **grafana-admins**).
