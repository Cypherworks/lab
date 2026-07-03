# blocky

DNS frontend that applies blocklists, allowlists and local records, then forwards to a local recursive resolver.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu on the target (systemd, apt, `systemd-resolved`). Defaults ship an `arm64` binary for Raspberry Pi; set `blocky_arch: amd64` for x86 nodes.
- Ansible `ansible.builtin` only. No external collections.
- Outbound HTTPS to `github.com` to fetch the pinned blocky release tarball, plus reachability to whatever blocklist URLs the inventory supplies.
- A recursive resolver reachable at `blocky_upstreams` (the `unbound` role on `127.0.0.1:5335`). The blocky unit orders itself `After=unbound.service`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `blocky_version` | `v0.32.1` | Pinned blocky release tag; selects the downloaded tarball. |
| `blocky_arch` | `arm64` | Release architecture. Use `amd64` on x86 nodes. |
| `blocky_install_dir` | `/opt/blocky` | Install root; versioned subdir plus a `blocky` symlink and `config.yml`. |
| `blocky_archive_name` | `blocky_{{ blocky_version }}_Linux_{{ blocky_arch }}.tar.gz` | Release archive filename. |
| `blocky_checksum` | `sha256:https://.../{{ blocky_version }}/blocky_checksums.txt` | Integrity check for the archive. A `sha256:<url>` verifies against blocky's published checksums file; override with a literal `sha256:<hash>` to pin, or `""` to skip. |
| `blocky_user` | `blocky` | System user the service runs as. |
| `blocky_dns_port` | `53` | DNS listen port. |
| `blocky_http_port` | `4000` | HTTP port for the query API and Prometheus `/metrics`. |
| `blocky_upstreams` | `[127.0.0.1:5335]` | Upstream resolvers blocky forwards to (the local unbound). |
| `blocky_bootstrap` | `tcp+udp:127.0.0.1:5335` | Resolver used to bootstrap blocklist URL downloads. |
| `blocky_block_type` | `zeroIp` | Response returned for a blocked name. |
| `blocky_block_ttl` | `1m` | TTL on blocked responses. |
| `blocky_cache_min` | `5m` | Minimum cache time. |
| `blocky_cache_max` | `30m` | Maximum cache time. |
| `blocky_log_level` | `info` | Log level. |
| `blocky_denylists` | `{}` | Site data. Map of group name to a list of blocklist URLs or inline entries. |
| `blocky_allowlists` | `{}` | Site data. Map of group name to a list of allowlist URLs or inline entries. |
| `blocky_client_groups_block` | `{default: []}` | Site data. Maps clients to the denylist groups that apply to them. |
| `blocky_custom_dns` | `{}` | Site data. Map of FQDN to IP for local lab records. |
| `blocky_disable_resolved_stub` | `true` | Free port 53 by disabling the `systemd-resolved` stub listener and relinking `/etc/resolv.conf`. |

Empty-collection defaults (`blocky_denylists`, `blocky_allowlists`, `blocky_custom_dns`) hold site data and are meant to be supplied from `group_vars` so every DNS node is identical.

## Dependencies

None.

## What it does

1. When `blocky_disable_resolved_stub` is set: creates `/etc/systemd/resolved.conf.d`, writes `10-no-stub.conf` with `DNSStubListener=no` (`0644`, notifies a `systemd-resolved` restart), and relinks `/etc/resolv.conf` to `/run/systemd/resolve/resolv.conf`. This frees port 53 for blocky.
2. Creates the `blocky_user` system user (`nologin`, no home created) and the versioned install directory.
3. Downloads the pinned blocky archive from GitHub, verified against `blocky_checksum`, then extracts it into `{{ blocky_install_dir }}/{{ blocky_version }}` (idempotent via `creates`).
4. Symlinks `{{ blocky_install_dir }}/blocky` to the current version's binary (notifies a restart).
5. Renders `config.yml` (`0640`, group `blocky_user`) from `config.yml.j2` and installs `/etc/systemd/system/blocky.service` (`0644`), both notifying a restart.
6. Enables and starts the service with `daemon_reload`.

Handlers: `Restart systemd-resolved` and `Restart blocky`. The systemd unit grants `CAP_NET_BIND_SERVICE` so the unprivileged user can bind port 53, and runs under `ProtectSystem=strict` / `ProtectHome=true` with `ReadWritePaths` limited to the install dir.

## Example

```yaml
- hosts: dns_nodes
  roles:
    - role: blocky
      vars:
        blocky_arch: arm64
        blocky_denylists:
          ads:
            - https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts
        blocky_client_groups_block:
          default:
            - ads
        blocky_custom_dns:
          caddy.lab.example: 10.200.30.10
```

## Notes

- The role rewrites `/etc/resolv.conf` and disables the resolved stub. On a host that other services expect to resolve through `127.0.0.53`, confirm this is acceptable before running.
- `blocky_http_port` serves both the query API and Prometheus metrics. There is no auth on that port; restrict it with host firewalling if the node is not on a trusted segment.
- The archive is verified against blocky's published checksums file by default (`blocky_checksum`). This catches transport corruption and tampering of the artifact; for a stronger guarantee against a compromised release, override `blocky_checksum` with a literal `sha256:<hash>` pinned to a reviewed version.
