# node_exporter

Installs and configures Prometheus node_exporter on hosts and Incus instances, bound to the lab address.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian-based host (`prometheus-node-exporter` comes from apt).
- Root via `become`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `node_exporter_listen_address` | `{{ ansible_default_ipv4.address }}` | Address the exporter binds to. The host's lab address, not `0.0.0.0`, so metrics are reachable only from the lab network. |
| `node_exporter_port` | `9100` | TCP port the exporter listens on. |
| `node_exporter_extra_args` | `""` | Extra flags appended to the exporter command line (e.g. to enable or disable collectors). |
| `node_exporter_apt_info_override` | `true` | Run the role's `apt_info.py` in place of the collectors', so `apt_upgrades_pending` is written. Set `false` where the package's own script counts pending upgrades; the role then removes its script and drop-in. |

## Dependencies

None.

## What it does

Installs the Debian `prometheus-node-exporter` package (which ships a hardened systemd unit) and its two Recommends, `prometheus-node-exporter-collectors` and `dbus`, by name, so a host that skips Recommends still gets the apt metrics and the systemd collector. It starts dbus, since a fresh install leaves it down until reboot. It writes `/etc/default/prometheus-node-exporter` to set the listen address and port, then enables and starts the service. A change to the defaults file restarts the exporter.

It also installs a systemd drop-in override (`node-exporter-override.conf.j2`) with `After=network-online.target` and `Restart=on-failure`. The exporter binds the host's lab address, which doesn't exist until the network is up, so ordering it after the network and retrying on failure stops a reboot from leaving it dead (and the host showing down in monitoring).

The collector set is left at the package default, which includes `hwmon` and `thermal_zone`. That gives CPU temperature on both the x86 nodes (coretemp via hwmon) and the ARM Pis (thermal_zone).

An unprivileged container sees its host's `/sys/class/hwmon` and reports the host's storage pool as its root, so the defaults there would repeat the host's temperatures and disk usage under the instance's name. Run instances with `--collector.disable-defaults` and only what the host and Incus can't see from outside: `textfile` (apt), `os`, `pressure` and `systemd`. lxcfs gives the container its own `/proc/pressure/{cpu,io,memory}` but no `irq`, and node_exporter 1.9.0 reads `irq` too, so `node_scrape_collector_success{collector="pressure"}` reads 0 there while the cpu, io and memory series still come through.

Apt metrics come from the `prometheus-node-exporter-collectors` package: its `prometheus-node-exporter-apt.timer` runs `apt_info.py` every 15 minutes and writes `/var/lib/prometheus/node-exporter/apt.prom`, which the textfile collector serves. That file holds `node_reboot_required`, `apt_upgrades_pending{origin,arch}`, `apt_upgrades_held`, `apt_autoremove_pending` and `apt_package_cache_timestamp_seconds`. Ubuntu 24.04 ships the collectors at `0.0~git20231018.f5c56e7-1`, whose `apt_info.py` counts `cache.get_changes()` without marking an upgrade first, so `apt_upgrades_pending` is never written. With `node_exporter_apt_info_override` on, the role installs `files/apt_info.py` as `/usr/local/sbin/prometheus-node-exporter-apt-info`, and a drop-in points the apt service at it. That file is Debian 13's `0.0~git20241119.a2b43e1-1` copy (Apache-2.0, from [node-exporter-textfile-collector-scripts](https://github.com/prometheus-community/node-exporter-textfile-collector-scripts)), which counts `is_upgradable`; only two comments are reworded. When nothing is pending, the script writes no `apt_upgrades_pending` series at all. The script reads the apt lists and never refreshes them, so the counts are only as fresh as the last `apt update` (`APT::Periodic::Update-Package-Lists`, set by the `unattended_upgrades` role).

`tests/apt_metrics.yml` boots Ubuntu 24.04 under systemd in Docker, offers one upgrade from a local repo, and reads the counts through node_exporter: with the override, with the package's script (which misses it), and with nothing pending. `tests/instance_profile.yml` boots Debian 13 with node_exporter installed without its Recommends, runs the instance collector set with the override off, and proves the package's `apt_info.py` counts an upgrade, only those collectors run, and no collector unit fails. Both need Docker, `community.docker`, and internet for the image and its apt packages.

## Example

```yaml
- hosts: bare_metal
  roles:
    - role: node_exporter
```

On Debian 13 Incus instances:

```yaml
- hosts: instances
  roles:
    - role: node_exporter
      vars:
        node_exporter_apt_info_override: false
        node_exporter_extra_args: >-
          --collector.disable-defaults --collector.textfile --collector.os
          --collector.pressure --collector.systemd
```

With a non-default port and an extra collector flag:

```yaml
- hosts: bare_metal
  roles:
    - role: node_exporter
      vars:
        node_exporter_port: 9101
        node_exporter_extra_args: "--collector.systemd"
```

## Notes

Metrics are unauthenticated; the lab network and host firewall are the access boundary. Binding to the lab address (not `0.0.0.0`) keeps the endpoint off any other interface.
