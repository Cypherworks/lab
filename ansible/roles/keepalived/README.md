# keepalived

VRRP floating VIP across the DNS nodes, with a real DNS-query health check that sheds the VIP when the local resolver stops answering.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu on the target (apt, systemd). Installs `keepalived` and `dnsutils` (for `dig`).
- Ansible `ansible.builtin` only. No external collections.
- Two or more nodes sharing an L2 segment on `keepalived_interface`, running a local resolver reachable at `127.0.0.1:53` (the `blocky` role).

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `keepalived_interface` | `eth0` | Interface the VRRP instance binds and the VIP attaches to. |
| `keepalived_vrid` | `20` | VRRP virtual router ID. Must match across peers and be unique on the segment. |
| `keepalived_vip` | `""` | Required, from inventory. The shared floating VIP. |
| `keepalived_state` | `BACKUP` | Per-host (`host_vars`). One node `MASTER`, the rest `BACKUP`. |
| `keepalived_priority` | `100` | Per-host (`host_vars`). Higher wins the VIP. |
| `keepalived_auth_pass` | `""` | Required, from SOPS. Shared VRRP authentication secret. |
| `keepalived_check_command` | `/etc/keepalived/check-dns.sh` | Health-check script path (deployed by this role). |
| `keepalived_check_weight` | `-50` | Priority delta applied when the check fails. See the note on the zombie-master constraint. |

`keepalived_vip` and `keepalived_auth_pass` are empty by default and must be supplied. `keepalived_state` and `keepalived_priority` differ per host and belong in `host_vars`.

## Dependencies

None.

## What it does

1. Installs `keepalived` and `dnsutils`.
2. Deploys `/etc/keepalived/check-dns.sh` (`0755`) from `check-dns.sh.j2`. The script runs `dig +tries=1 +time=2 +norecurse @127.0.0.1 . NS`; a non-zero exit (no reply within the timeout) drops this node's VRRP priority. `+norecurse` keeps it a pure liveness test so an upstream outage does not force a needless failover. Notifies a restart.
3. Renders `/etc/keepalived/keepalived.conf` (`0640`) from `keepalived.conf.j2`: a `vrrp_script chk_dns` (interval 5, fall 2, rise 2, weight `keepalived_check_weight`) tracked by a `vrrp_instance VI_DNS`. Notifies a restart.
4. Enables and starts the service.

Handler: `Restart keepalived`.

## Example

```yaml
- hosts: dns_nodes
  vars:
    keepalived_vip: 10.200.20.2
    keepalived_auth_pass: "{{ vault_keepalived_pass }}"
  roles:
    - role: keepalived
# host_vars/dns1: keepalived_state=MASTER, keepalived_priority=150
# host_vars/dns2: keepalived_state=BACKUP, keepalived_priority=100
```

## Notes

- The health check queries the resolver rather than checking `systemctl is-active`, so a hung (not merely stopped) resolver also sheds the VIP.
- The magnitude of `keepalived_check_weight` must exceed the MASTER-to-BACKUP priority gap. With priorities 150 and 100 (gap 50) a weight of `-50` only ties the backup, so the failed master keeps the VIP: the zombie-master bug. Set the weight from the deploy to match the real priority spread (a value greater than the gap).
- `keepalived_auth_pass` is a shared VRRP secret. Keep it in SOPS, not in plain inventory.
