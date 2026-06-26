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

Pairs with the `node_exporter` host role (CPU/mem/disk/**temperature**). Day-one
alerts: high CPU temp, target down, disk > threshold, Patroni no-primary, Redis
down — all thresholds tunable from the deploy.

Grant a user Grafana by adding them to **grafana-users** (Admin via **grafana-admins**).
