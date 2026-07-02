# unbound

Recursive, validating DNS resolver bound to loopback, sitting behind the local DNS frontend.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu on the target (apt, systemd). Installs the `unbound` and `dns-root-data` packages.
- Ansible `ansible.builtin` only. No external collections.
- A local DNS frontend (the `blocky` role) to forward queries here; unbound listens on loopback only and is not exposed to the network.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `unbound_listen` | `127.0.0.1` | Interface unbound binds. Loopback only by design. |
| `unbound_port` | `5335` | Listen port. The frontend forwards here. |
| `unbound_cache_min_ttl` | `300` | Minimum cache TTL (seconds). |
| `unbound_cache_max_ttl` | `86400` | Maximum cache TTL (seconds). |
| `unbound_edns_buffer_size` | `1232` | Advertised EDNS UDP buffer size. |

## Dependencies

None.

## What it does

1. Installs `unbound` and `dns-root-data` (the latter provides `/usr/share/dns/root.hints` and the DNSSEC root anchor).
2. Renders `/etc/unbound/unbound.conf.d/lab.conf` (`0644`) from `lab.conf.j2`, validated with `unbound-checkconf %s` before it is written. Notifies a restart.
3. Enables and starts the `unbound` service.

The config binds `interface: {{ unbound_listen }}@{{ unbound_port }}`, refuses all access except `127.0.0.0/8`, and enables DNSSEC hardening, QNAME minimisation, aggressive NSEC and prefetching. It supplies fresher root hints from `dns-root-data` but deliberately does not declare an `auto-trust-anchor-file`: the unbound package already seeds and RFC 5011-maintains `/var/lib/unbound/root.key`, and declaring the anchor twice is a fatal "trust anchor presented twice" error.

Handler: `Restart unbound`.

## Example

```yaml
- hosts: dns_nodes
  roles:
    - role: unbound
    - role: blocky   # frontend that forwards to 127.0.0.1:5335
```

## Notes

- `validate: unbound-checkconf` gates the template write, so a broken config fails the task rather than reloading a running resolver into a bad state.
- The resolver answers on loopback only. It is meant to sit behind blocky, not to serve clients directly.
- The defaults file comment still refers to "AdGuard Home" as the frontend; the actual frontend in this library is the `blocky` role. Functionally identical (both forward to loopback), but the comment is stale.
