# monitoring

The observability stack on one Incus instance (docker compose):

- **VictoriaMetrics (`vmsingle`)** — metrics storage + scraping (PromQL, lighter
  than Prometheus, better compaction). Scrape targets come from the deploy.
- **vmalert** — evaluates the alert rules against vmsingle.
- **Alertmanager → alertmanager-ntfy → ntfy** — alerts pushed to your phone via a
  self-hosted ntfy (no third party). *The bridge config/image is unverified — see
  the inline NOTE in `am-ntfy.yml.j2` and confirm on apply.*
- **Grafana** — VictoriaMetrics datasource + provisioned dashboards, SSO via
  Authentik (the `grafana` blueprint + `grafana-users` group). Two single-screen
  1920x1080 no-scroll dashboards (`Lab Overview`, `Lab Services & NAS`), each with
  the **Front CCTV** strip pinned at the top and tagged `kiosk`. The role creates a
  **kiosk playlist** (via the Grafana API — the file provisioner can't) that rotates
  every `kiosk`-tagged dashboard; the hosts block is a single threshold-coloured
  table (one row per host). The CCTV feed is a placeholder until task #18.
- **blackbox_exporter** — https probes for TLS cert expiry
  (`probe_ssl_earliest_cert_expiry`). Targets: `monitoring_blackbox_targets`
  (https URLs). Module `https_2xx`.
- **snmp_exporter** — the Synology NAS over **SNMPv3 (authPriv, SHA+AES)**. A
  hand-written `snmp.yml` walks only the OIDs we need (system status/temp, per-disk
  temp/status, RAID volume free/total). Target: `monitoring_snmp_targets`. The v3
  auth/priv passwords come from the container env (`.env`) and are expanded at load
  time (`--config.expand-environment-variables`), so no plaintext lands on disk.
- **go2rtc** — RTSP→MSE restream for the CCTV strip. Two streams (`front`,
  `garage_carport`); their RTSP source URLs carry Scrypted access tokens, so they
  come from `.env` and go2rtc expands the `${...}` placeholders in `go2rtc.yaml` at
  load time (native env-var templating, no flag), so no plaintext token lands on
  disk. The `cctv` dashboard (tagged `kiosk`, so it joins the playlist) embeds the
  **Front** camera as a top strip via a Grafana text panel in HTML mode pointing an
  `<iframe>` at go2rtc's `/stream.html?src=front&mode=mse`. That iframe renders only
  because the role sets `GF_PANELS_DISABLE_SANITIZE_HTML=true` (scoped to text-panel
  HTML; Grafana itself stays behind Authentik OIDC — this is **not** `allow_embedding`,
  which is about embedding Grafana elsewhere). The browser loads the stream directly,
  so it must reach go2rtc over HTTPS via a Caddy front (`monitoring_go2rtc_public_url`),
  never the internal compose port (mixed content). `mode=mse` keeps it to one HTTP/WS
  port that proxies cleanly through Caddy; WebRTC (UDP/TCP 8555) is left for on-LAN
  use and not published. go2rtc Basic auth (`monitoring_go2rtc_username/password`) is
  off by default: it wraps the whole UI/API including `stream.html`, so enabling it
  would make the browser prompt for credentials inside the iframe and break the embed.
  With it off, anyone who can reach the monitoring host (or the Caddy route) on the
  lab can open the go2rtc UI — the access boundary is lab-only reachability + Caddy TLS.

Scrape targets are `{target: "ip:port", instance: "hostname"}` mappings supplied
by the deploy, so series carry the real hostname as their `instance` label
(dashboards read hostnames, not IPs). No per-node node_exporter change. Headscale
is scraped over the overlay via `monitoring_headscale_target` (its tailnet IP:9090).

Pairs with the `node_exporter` host role (CPU/mem/disk/**temperature**). Day-one
alerts: high CPU temp, target down, disk > threshold, Patroni no-primary, Redis
down — all thresholds tunable from the deploy.

Grant a user Grafana by adding them to **grafana-users** (Admin via **grafana-admins**).
