# node_exporter

Installs and configures Prometheus node_exporter on bare-metal hosts, bound to the lab address.

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

## Dependencies

None.

## What it does

Installs the Debian `prometheus-node-exporter` package (which ships a hardened systemd unit), writes `/etc/default/prometheus-node-exporter` to set the listen address and port, then enables and starts the service. A change to the defaults file restarts the exporter.

The collector set is left at the package default, which includes `hwmon` and `thermal_zone`. That gives CPU temperature on both the x86 nodes (coretemp via hwmon) and the ARM Pis (thermal_zone).

## Example

```yaml
- hosts: bare_metal
  roles:
    - role: node_exporter
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
