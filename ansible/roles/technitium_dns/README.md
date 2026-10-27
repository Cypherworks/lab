# technitium_dns

Installs [Technitium DNS Server](https://technitium.com/dns/) on Ubuntu and holds its server settings to code through its HTTP API. Recursion, the recursion client ACL, forwarders and the HTTPS console are managed by the role. Zones and records are left to the web console. A host firewall (ufw) lets DNS in only from the client networks, and SSH and the console only from the admin sources.

Part of the `lab` mechanism library: a generic, parameterised role. Supply site data (the admin password, ACL, forwarders, firewall sources) from your inventory and SOPS, not from the role.

## How it works

1. `install.yml`: installs the ASP.NET Core runtime from the distro's signed apt (`aspnetcore-runtime-10.0`), creates the `dns-server` service user, and downloads the pinned release on the control host, checking its sha256 against the `.sha256` Technitium publishes beside each archive. It then copies and unpacks the release into `/opt/technitium/dns` when the installed version differs, and installs a systemd unit that mirrors upstream's (`ProtectSystem=strict`, write access only to `/etc/dns` and the log dir, `CAP_NET_BIND_SERVICE`), run with the distro `dotnet`. It doesn't start the service.
2. `configure.yml`: brings up the firewall before the first start, so the default admin password is never reachable from the network. It refuses to enable ufw if the run's own SSH source isn't an admin source. It then starts the service and, on loopback only, logs in. On a fresh install it replaces the default `admin`/`admin` password with `technitium_dns_admin_password`. Finally it reads the settings and writes only what differs: `recursion`, `recursionNetworkACL`, `forwarders`, `forwarderProtocol`, and HTTPS on `technitium_dns_https_port` with a self-signed certificate. A rerun with nothing to change reports no change.
3. `main.yml` runs both.

Rebuilds (fresh VM, same records): `archive_config.yml` stops the old server and pulls its whole `/etc/dns` to the control host. `restore_config.yml` lays it down on the new server after `install.yml` and before `configure.yml`, as an in-place upgrade does. Technitium updates its own files on start.

Zone backups (the server lost, not replaced): `backup_zones.yml` takes a zones-only backup through the API (`/api/settings/backup?zones=true`) while the server keeps answering, and pulls it to the control host as `<host>-zones-<UTC time>.zip` (0600, in a 0700 dir), keeping the newest `technitium_dns_backup_keep`. `restore_zones.yml` restores the newest of those through `/api/settings/restore`, but only into a server with no zones of its own (Technitium's internal ones don't count), so a converge never overwrites console changes. It uploads with `curl` because `uri` base64-encodes multipart files, which Technitium can't read. Technitium writes a zone change to disk about 5 seconds after it's made, so a backup taken straight after an edit misses that edit. `tests/zone_backups.yml` exercises both against a throwaway container on loopback.

The plain-HTTP port (5380) is used by the role on loopback and is closed to the network by the firewall; the console is HTTPS on 53443.

## Requirements

- Ubuntu with `aspnetcore-runtime-10.0` in its archive (24.04 and later).
- The control host can reach `download.technitium.com`; the target needn't have internet.
- `community.general` (ufw).
- `curl` on the target, for `restore_zones.yml`.

## Role variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `technitium_dns_version` | `15.6.2` | Pinned release. |
| `technitium_dns_archive_sha256` | the 15.6.2 sha256 | Pinned checksum of that release. |
| `technitium_dns_archive_url` | the archive URL for the version | Release tarball. |
| `technitium_dns_cache_dir` | `~/.cache/technitium_dns` | Release cache on the control host. |
| `technitium_dns_runtime_package` | `aspnetcore-runtime-10.0` | Runtime package. |
| `technitium_dns_dotnet` | `/usr/bin/dotnet` | dotnet host the unit runs; checked after install. |
| `technitium_dns_install_dir` / `_config_dir` / `_log_dir` | `/opt/technitium/dns`, `/etc/dns`, `/var/log/technitium/dns` | Paths. |
| `technitium_dns_http_port` / `_https_port` | `5380` / `53443` | Loopback API / console. |
| `technitium_dns_admin_password` | `""` | Admin password (SOPS, 16+ characters). |
| `technitium_dns_recursion` | `Deny` | `Deny`, `Allow`, `AllowOnlyForPrivateNetworks` or `UseSpecifiedNetworkACL`. |
| `technitium_dns_recursion_acl` | `[]` | Who may recurse with `UseSpecifiedNetworkACL` (`!` denies). |
| `technitium_dns_forwarders` | `[]` | Forwarders; empty clears them. |
| `technitium_dns_forwarder_protocol` | `Udp` | `Udp`, `Tcp`, `Tls`, `Https` or `Quic`. |
| `technitium_dns_firewall` | `true` | Manage ufw. |
| `technitium_dns_dns_clients` | `[]` | Sources allowed to query on 53. |
| `technitium_dns_admin_sources` | `[]` | Sources allowed SSH and the console. |
| `technitium_dns_archive_dir` | `~/.local/state/technitium_dns` | Rebuild archives on the control host. |
| `technitium_dns_restore_from` | `""` | `restore_config.yml`: whose archive to restore. |
| `technitium_dns_backup_keep` | `30` | Zone backups kept per host in `technitium_dns_archive_dir`. |
