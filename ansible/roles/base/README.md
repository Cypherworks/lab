# base

OS baseline for every Ubuntu lab host: hostname, base packages, timezone, the
ops SSH keys for the `ansible` user, and the static lab network (netplan).

Time sync is **not** managed here — the CIS hardening layer (rule 2.3.x) enforces
a single time daemon (chrony) and would tear out anything base configured. Set
NTP via `ubtu24cis_time_pool` / `ubtu24cis_time_servers` in the inventory.

## Required host/inventory vars
- `lab_ip`, `lab_gateway` — host address + gateway (per host_vars).
- `ops_ssh_authorized_keys` — list of SSH public keys for the `ansible` user.
- `timezone`, `dns_vip`, `internal_subdomain` — from group_vars.

## Optional (defaults shown in `defaults/main.yml`)
- `lab_interface` (eth0), `lab_prefix` (24), `lab_nameservers` (`[dns_vip]`),
  `lab_search`, `base_packages`.

The DNS Pis set `lab_nameservers: [1.1.1.1]` (they resolve upstream, not via the
VIP they themselves back). Re-applying the host's existing IP is a no-op, so a
re-run won't drop the SSH connection.
