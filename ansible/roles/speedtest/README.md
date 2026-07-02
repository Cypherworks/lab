# speedtest

Runs Speedtest Tracker on one Incus instance via docker compose, testing the upstream line on a schedule.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- `community.docker` collection.
- A container/host the `docker` role can install Docker Engine and Compose v2 onto.
- `speedtest_app_key` supplied from SOPS.
- Root via `become`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `speedtest_compose_dir` | `/opt/speedtest` | Host directory for the compose file, `.env`, and the SQLite `/config` volume. |
| `speedtest_image` | `lscr.io/linuxserver/speedtest-tracker:v1.14.5-ls159` | Pinned container image. Confirm the registry tag exists before apply. |
| `speedtest_http_port` | `8080` | Host port mapped to the container's HTTP (`:80`); Caddy reverse-proxies to this. |
| `speedtest_app_url` | `""` | Public URL Caddy serves the app on; used to build absolute links. Set by the deploy. |
| `speedtest_schedule` | `"0 */6 * * *"` | Cron schedule for running Ookla tests. Default is every 6 hours. |
| `speedtest_timezone` | `"Etc/UTC"` | Container timezone. |
| `speedtest_puid` | `1000` | UID the LinuxServer image drops to. |
| `speedtest_pgid` | `1000` | GID the LinuxServer image drops to. |
| `speedtest_app_key` | `""` | Laravel `APP_KEY` that encrypts stored data. A secret from SOPS; generate with `openssl rand -base64 32` prefixed with `base64:`. |

## Dependencies

- `docker` role (meta dependency) — installs Docker Engine and Compose v2.

## What it does

Creates `speedtest_compose_dir`, renders the `.env` (carrying the `APP_KEY` secret, mode `0600`) and the compose file, then brings the stack up with `docker_compose_v2`. Speedtest Tracker periodically runs Ookla speedtests and graphs upload, download, and latency over time. It is SQLite-backed (a single `/config` volume), so it carries no dependency on the lab's Patroni data tier. It exposes a native `/prometheus` endpoint for scraping.

Changes to the `.env` or the compose file restart the stack.

## Example

```yaml
- hosts: speedtest_host
  roles:
    - role: speedtest
      vars:
        speedtest_app_url: "https://speedtest.example.com"
        speedtest_app_key: "{{ vault_speedtest_app_key }}"
```

## Notes

The native `/prometheus` endpoint (on the app's HTTP port) is what the `monitoring` role scrapes via `monitoring_speedtest_targets`.
