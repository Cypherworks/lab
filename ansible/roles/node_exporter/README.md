# node_exporter

Prometheus `node_exporter` (Debian's `prometheus-node-exporter`) on bare-metal
hosts — CPU, memory, disk, and **temperature**. The default collectors include
`hwmon` (x86 coretemp) and `thermal_zone` (the Pis), so CPU temps are exposed on
both architectures with no extra config — `node_hwmon_temp_celsius` on the
ThinkCentres, `node_thermal_zone_temp` on the Pis.

Binds the host's lab IP only (`node_exporter_listen_address`); access is gated by
the lab network and host firewall. Scraped by the `monitoring` instance.

Used by the cluster nodes and DNS Pis (see the deploy's `node.yml` / `dns.yml`).
