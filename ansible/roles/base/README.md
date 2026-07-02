# base

Baseline OS configuration applied to every Ubuntu host: hostname, base packages, timezone, ops SSH keys, and optional static lab networking.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Ubuntu (Debian-family, netplan-based networking).
- Collections: `community.general` (`timezone`), `ansible.posix` (`authorized_key`). Core `ansible.builtin` modules (`hostname`, `apt`, `template`, `command`) cover the rest.
- Privilege escalation (`become`) to root.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `base_packages` | `[curl, ca-certificates, unzip, htop, vim, python3]` | Packages installed on every host. |
| `lab_interface` | `eth0` | Primary NIC for the static lab address. Pis use `eth0`; x86 NICs are set per host. |
| `lab_prefix` | `24` | CIDR prefix length for `lab_ip`. |
| `lab_nameservers` | `["{{ dns_vip }}"]` | Resolver addresses written to the netplan config. Depends on inventory `dns_vip`. |
| `lab_search` | `"{{ internal_subdomain }}"` | DNS search domain. Depends on inventory `internal_subdomain`. |
| `lab_vlans` | `[]` | Optional tagged VLAN sub-interfaces. Each entry: `name`, `id`, `addresses`, and optional `link` (defaults to `lab_interface`). No gateway is assigned. |
| `ops_ssh_authorized_keys` | `[]` | SSH public keys authorised for the ansible user. Config, not secret; supplied by inventory. |

Required from inventory, no default (network config is only templated when `lab_ip` is defined):

- `lab_ip` — host static address. Its presence is the switch that renders the static-network config; hosts without it keep platform/DHCP networking.
- `lab_gateway` — default-route gateway.
- `timezone` — passed to the `timezone` module.
- `dns_vip`, `internal_subdomain` — consumed by the `lab_nameservers` / `lab_search` defaults.
- `ansible_user` — the account that ops keys are authorised for.
- `lab_interface_match` (optional) — if defined, adds a netplan `match: name:` block for stable NIC matching.

No secrets are consumed by this role.

## Dependencies

None.

## What it does

1. Sets the system hostname to `inventory_hostname`.
2. Installs `base_packages` via apt (cache refreshed, `cache_valid_time: 3600`).
3. Sets the timezone to `timezone`.
4. Authorises each key in `ops_ssh_authorized_keys` for `ansible_user`.
5. When `lab_ip` is defined, templates `lab-netplan.yaml.j2` to `/etc/netplan/60-lab.yaml` (root:root, mode `0600`): a static v4 address on `lab_interface` with DHCP disabled, a default route via `lab_gateway`, nameservers and search domain, plus any `lab_vlans` sub-interfaces. Changes to the file notify the `Apply netplan` handler, which runs `netplan apply`.

NTP/time sync is deliberately not handled here. The CIS hardening layer (CIS 2.3.x) owns the single time daemon (chrony) and would remove anything base configured; NTP servers are set via `ubtu24cis_time_pool`/`servers` in the inventory.

## Example

```yaml
- hosts: lab_nodes
  roles:
    - role: base
      vars:
        timezone: Europe/London
        lab_ip: 10.200.20.15
        lab_gateway: 10.200.20.1
        ops_ssh_authorized_keys:
          - "ssh-ed25519 AAAA... ops@admin"
```

## Notes

Re-asserting a host's existing static IP is a no-op, so a re-run does not drop the connection; `netplan apply` only fires when the rendered file actually changes.

Cloud-hosted hosts (e.g. the Headscale EC2) omit `lab_ip` and keep their platform-assigned networking untouched.

The netplan file is written mode `0600` because it is root-only network config. `lab_vlans` adds tagged sub-interfaces with addresses but no gateway, used where a host needs an extra VLAN foot (the Pi witness gaining a VLAN-30 address for quorum) without disturbing its primary interface.
