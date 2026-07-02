# unattended_upgrades

Configures `unattended-upgrades` to apply security-pocket updates nightly and reboot in a quiet window when required.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Ubuntu/Debian host with the `unattended-upgrades` and `apt-listchanges` packages available.
- Core `ansible.builtin` modules only: `apt` and `template`.
- Privilege escalation (`become`) to root.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `unattended_upgrades_origins` | `["${distro_id}:${distro_codename}-security", "${distro_id}ESMApps:${distro_codename}-apps-security", "${distro_id}ESM:${distro_codename}-infra-security"]` | Allowed origins written to the policy; the ESM entries cover the Ubuntu apps/infra security pockets. |
| `unattended_upgrades_auto_reboot` | `true` | Whether to reboot automatically after an update that needs it. |
| `unattended_upgrades_auto_reboot_time` | `"03:30"` | Time of day for the automatic reboot. |
| `unattended_upgrades_remove_unused_deps` | `true` | Whether to remove unused dependencies after upgrades. |

No inventory data or secrets are required.

## Dependencies

None.

## What it does

1. Installs `unattended-upgrades` and `apt-listchanges` via apt (cache refreshed, `cache_valid_time: 3600`).
2. Templates `20auto-upgrades.j2` to `/etc/apt/apt.conf.d/20auto-upgrades` (root:root, mode `0644`), enabling periodic package-list updates, unattended upgrades, download of upgradeable packages, and a 7-day autoclean interval.
3. Templates `50unattended-upgrades.j2` to `/etc/apt/apt.conf.d/50unattended-upgrades` (root:root, mode `0644`) with the allowed origins, unused-dependency removal, and automatic-reboot policy.

## Example

```yaml
- hosts: all
  roles:
    - role: unattended_upgrades
      vars:
        unattended_upgrades_auto_reboot_time: "04:00"
```

## Notes

This is the free stand-in for Ubuntu Pro Livepatch/ESM: it applies the security pocket automatically rather than providing live kernel patching.

The `${distro_id}`/`${distro_codename}` tokens are apt's own variables, expanded by unattended-upgrades at run time, not by Jinja. Automatic reboots occur at the configured time and will interrupt running workloads; adjust or disable per host group where that is unacceptable.
