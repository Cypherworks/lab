# keepalived

Installs keepalived and configures a single VRRP floating VIP for an active/passive service pair. The VIP floats to whichever member is MASTER; an optional health-check command demotes a member whose service has failed so the VIP moves to a healthy one.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply the VIP, per-host state/priority, and secrets from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu target with systemd.
- All members on the same L2 segment (VRRP is link-local multicast). In Incus, bridged containers on the same VLAN bridge qualify; each adds the VIP to its own `eth0`, which works in an unprivileged container.
- The service that binds the VIP must listen on all addresses (e.g. `0.0.0.0:443`) so it answers on the VIP the moment keepalived adds it.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `keepalived_vip` | `""` (required) | The floating address. |
| `keepalived_auth_pass` | `""` (required, SOPS) | VRRP authentication password, shared by the pair. |
| `keepalived_state` | `BACKUP` | `MASTER` on one member, `BACKUP` on the rest. |
| `keepalived_priority` | `100` | Higher wins the VIP. MASTER higher than BACKUP. |
| `keepalived_interface` | `eth0` | Interface carrying the VIP. |
| `keepalived_vrid` | `51` | Virtual router ID; unique per VRRP domain on the segment. |
| `keepalived_instance` | `VI_1` | VRRP instance name. |
| `keepalived_check_command` | `""` | Optional health check; exit 0 when healthy. Empty disables the check. |
| `keepalived_check_weight` | `-60` | Priority shed on check failure. Must exceed the MASTER/BACKUP gap so a failed MASTER drops below a healthy BACKUP. |
| `keepalived_check_interval` / `_fall` / `_rise` | `2` / `2` / `2` | Check cadence and the consecutive fail/pass counts to change state. |

## Example

```yaml
# MASTER member (host_var priority 150, state MASTER); BACKUP member gets 100.
- hosts: caddy
  roles:
    - role: keepalived
      vars:
        keepalived_vip: 10.0.30.10
        keepalived_auth_pass: "{{ vault_caddy_vrrp_pass }}"
        keepalived_check_command: "systemctl is-active --quiet caddy"
```

With MASTER 150 / BACKUP 100 and weight -60, a MASTER whose caddy fails drops to 90, below the healthy BACKUP's 100, so the VIP fails over; it returns when caddy recovers.

## Notes

- The health-check weight must be larger than the MASTER/BACKUP priority gap, or a failed MASTER keeps the VIP. The default (-60 against a 50-point gap) satisfies this.
- keepalived runs the check as root (no `enable_script_security`), acceptable for a single-purpose HA container. Keep the check a simple, root-owned command.
