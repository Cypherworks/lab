# technitium_dns

Installs [Technitium DNS Server](https://technitium.com/dns/) on Ubuntu and holds its server settings to code through its HTTP API. Recursion, the recursion client ACL, forwarders, blocking, DNSSEC validation and the HTTPS console are managed by the role. So are any Conditional Forwarder zones you list. Every other zone and record is left to the web console. Run the `firewall` role before it: this role opens no ports of its own, and the service's default admin password must never be reachable from the network.

Part of the `lab` mechanism library: a generic, parameterised role. Supply site data (the admin password, ACL, forwarders) from your inventory and SOPS, not from the role.

## How it works

1. `install.yml`: installs the ASP.NET Core runtime from the distro's signed apt (`aspnetcore-runtime-10.0`), creates the `dns-server` service user, and downloads the pinned release on the control host, checking its sha256 against the `.sha256` Technitium publishes beside each archive. It then copies and unpacks the release into `/opt/technitium/dns` when the installed version differs, and installs a systemd unit that mirrors upstream's (`ProtectSystem=strict`, write access only to `/etc/dns` and the log dir, `CAP_NET_BIND_SERVICE`), run with the distro `dotnet`. It turns off the `systemd-resolved` stub listener and relinks `/etc/resolv.conf` to `/run/systemd/resolve/resolv.conf`, because the stub costs Technitium its IPv4 TCP/53 bind. It doesn't start the service.
2. `configure.yml`: starts the service, checks something answers TCP on `127.0.0.1:53` (Technitium only logs a failed bind and carries on, and the API comes up either way), and, on loopback only, logs in. On a fresh install it replaces the default `admin`/`admin` password with `technitium_dns_admin_password`. `hold.yml` runs everything held through the API. `settings.yml` reads the settings and writes only what differs: `recursion`, `recursionNetworkACL`, `forwarders`, `forwarderProtocol`, `dnssecValidation`, `enableBlocking`, `blockingType`, `blockListUrls`, `blockListUpdateIntervalHours`, and HTTPS on `technitium_dns_https_port` with a self-signed certificate. It reads them back after a write and fails if they didn't take, because Technitium answers ok to some values it doesn't know (an unknown recursion policy, say) and keeps the old one. A web service change restarts that service 2 seconds after the write, dropping any request in flight, so the role waits it out. Then it forms the cluster, if one is set, and holds `technitium_dns_forwarder_zones`, when any are set. A rerun with nothing to change reports no change.
3. `main.yml` runs both.

Rebuilds (fresh VM, same records): `archive_config.yml` stops the old server and pulls its whole `/etc/dns` to the control host. `restore_config.yml` lays it down on the new server after `install.yml` and before `configure.yml`, as an in-place upgrade does. Technitium updates its own files on start.

Sign-in (`sso.yml`): setting `technitium_dns_sso_authority` turns on OpenID Connect, with sign-up limited to users in a group from `technitium_dns_sso_group_map`, a `remote group: local group` map. The role holds these on the primary, and they reach a secondary with the rest of its config. Group names come from the provider's `roles` claim. The local admin login keeps working, so it's the way in when the provider is down. The server masks the client secret when read, so changing only `technitium_dns_sso_client_secret` isn't seen as a change: rotate it in the console, or change another SSO setting in the same run. Clearing the authority leaves SSO as it is rather than turning it off. `tests/sso.yml` covers the settings, not a real sign-in.

`technitium_dns_web_reverse_proxy_addresses` lists the proxies trusted to name the client in a header. The server's own default trusts every private range, which would let any client on them set its logged address, so the role holds this and defaults to trusting none.

Forwarder zones (`forwarder_zones.yml`): each entry of `technitium_dns_forwarder_zones` is a Conditional Forwarder zone, so a server can answer some of a domain's names itself and send the rest on. Local records win per name and type, and anything else forwards, including other types at a name a wildcard covers. A wildcard does catch A queries for every name without a local A record, so a public name under one has to be listed in `passthrough`, which gives it its own FWD record. With `forwarder: this-server` the rest of the domain resolves through the server's own recursion. `query_acl` limits who gets any answer from the zone; other clients get REFUSED. DNSSEC validation is per zone, as forwarder zones skip the server's setting: `dnssec_validation` defaults to on with `this-server` and off with another server, since a private zone there can't validate. The role adds or corrects what's listed and leaves other names to the console, so dropping a name from the list doesn't remove it from the server. It refuses a zone of the same name that isn't a Forwarder zone, as a primary would hide the domain's public records from clients. `tests/forwarder_zones.yml` exercises all of this against throwaway containers, with no internet.

Blocking: `technitium_dns_block_list_urls` takes lists of domains or hosts-file entries, each name blocking its subdomains too. Technitium fetches them on its own after they change and on `technitium_dns_block_list_update_hours` after that. `technitium_dns_blocking_type` is held because the server's default in 15.6.2 is `NxDomain`, not the `AnyAddress` its API docs give. `tests/blocking.yml` exercises these settings, the read-back and a console change being put back against throwaway containers, with no internet.

Clusters: setting `technitium_dns_cluster_domain` makes this node a member of a cluster. The domain can't change afterwards without deleting the cluster, and it must not be a zone you use for anything else, since the cluster creates a primary zone with that name. Each node also needs a `technitium_dns_cluster_role` and its static `technitium_dns_cluster_node_ip`, and a secondary needs the primary's URL (`https://<primary hostname>.<cluster domain>:<https port>/`) and address. Run the primary first, one host at a time: a secondary's join fails until the primary is up. A secondary joins with the admin password, so every node shares `technitium_dns_admin_password`. The primary's certificate is self-signed, so that one call skips TLS verification and the password is exposed to anyone able to intercept traffic between the nodes at that moment; after it, nodes authenticate each other with DANE.

Joining overwrites a secondary's settings, users and apps with the primary's. Settings come first in `hold.yml` for a reason: a join that has to turn HTTPS on itself, while the primary has SSO on, hangs the server until it's restarted. After that the role writes nothing to it, not settings and not zones, because the primary owns them, even though Technitium answers ok to a settings write on a secondary. The role does refuse a node that's in another cluster or holds the other role.

On the primary, every zone in `technitium_dns_forwarder_zones` becomes a member of the cluster's catalog zone, including zones that existed before the cluster, and replicates to the secondary as a Secondary Forwarder zone, usually within seconds. The role sets `overrideCatalogQueryAccess` on each one: without it, the secondary answers clients the zone's `query_acl` refuses on the primary. Zones made in the console join the catalog only if you add them in their Zone Options, and need that same override if they restrict query access. With the primary down, the secondary keeps answering but nothing can be changed. `tests/cluster.yml` exercises all of this against throwaway containers, with no internet. The nodes need to reach each other on the HTTPS port and on DNS, so the firewall role's rules must let them.

Zone backups (the server lost, not replaced): `backup_zones.yml` takes a zones-only backup through the API (`/api/settings/backup?zones=true`) while the server keeps answering, and pulls it to the control host as `<host>-zones-<UTC time>.zip` (0600, in a 0700 dir), keeping the newest `technitium_dns_backup_keep`. `restore_zones.yml` restores the newest of those through `/api/settings/restore`, but only into a server with no zones of its own (Technitium's internal ones don't count), so a converge never overwrites console changes. It uploads with `curl` because `uri` base64-encodes multipart files, which Technitium can't read. Technitium writes a zone change to disk about 5 seconds after it's made, so a backup taken straight after an edit misses that edit. `tests/zone_backups.yml` exercises both against a throwaway container on loopback.

`tests/resolved_stub.yml` boots systemd in a throwaway container to exercise the stub listener, the port check and the fix against a real `systemd-resolved`.

The role uses the plain-HTTP port (5380) on loopback, so the firewall should keep it closed to the network; the console is HTTPS on 53443.

## Requirements

- Ubuntu with `aspnetcore-runtime-10.0` in its archive (24.04 and later), running `systemd-resolved`.
- The control host can reach `download.technitium.com`; the target needn't have internet.
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
| `technitium_dns_blocking` | `true` | Blocking on or off. |
| `technitium_dns_blocking_type` | `AnyAddress` | `AnyAddress` (0.0.0.0), `NxDomain` or `CustomAddress`. |
| `technitium_dns_block_list_urls` | `[]` | Block lists to fetch; empty clears them. |
| `technitium_dns_block_list_update_hours` | `24` | How often they're re-fetched. |
| `technitium_dns_dnssec_validation` | `true` | DNSSEC validation for recursion. |
| `technitium_dns_web_reverse_proxy_addresses` | `[]` | Proxies trusted to name the client in a header. |
| `technitium_dns_sso_authority` | `""` | OpenID Connect issuer; empty leaves SSO alone. |
| `technitium_dns_sso_client_id` | `""` | Client id at the provider. |
| `technitium_dns_sso_client_secret` | `""` | Client secret (SOPS). |
| `technitium_dns_sso_group_map` | `{}` | Provider group: local group. |
| `technitium_dns_forwarder_zones` | `[]` | Conditional Forwarder zones held to code. Each has `zone`, `forwarder` (an address or `this-server`), and optionally `records` (name: A address, `*` for a wildcard), `passthrough` (names to forward past a wildcard), `query_acl` (the only clients answered; empty lets anyone ask) and `dnssec_validation` (on for `this-server`, otherwise off). |
| `technitium_dns_cluster_domain` | `""` | Cluster domain; empty means no cluster. Can't change later. |
| `technitium_dns_cluster_role` | `primary` | `primary` or `secondary`. |
| `technitium_dns_cluster_node_ip` | `""` | This node's static address. |
| `technitium_dns_cluster_primary_url` | `""` | Secondary: the primary's HTTPS URL. |
| `technitium_dns_cluster_primary_ip` | `""` | Secondary: the primary's address. |
| `technitium_dns_archive_dir` | `~/.local/state/technitium_dns` | Rebuild archives on the control host. |
| `technitium_dns_restore_from` | `""` | `restore_config.yml`: whose archive to restore. |
| `technitium_dns_backup_keep` | `30` | Zone backups kept per host in `technitium_dns_archive_dir`. |
