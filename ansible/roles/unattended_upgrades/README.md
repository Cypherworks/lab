# unattended_upgrades

Automatic security updates via `unattended-upgrades` — the free stand-in for
Ubuntu Pro Livepatch/ESM. Applies the security pocket nightly and reboots in a
quiet window (default 03:30) when a reboot is required.

## Vars (defaults in `defaults/main.yml`)
- `unattended_upgrades_origins` — allowed upgrade origins (ESM lines are inert
  without a Pro subscription).
- `unattended_upgrades_auto_reboot` / `_auto_reboot_time`.
- `unattended_upgrades_remove_unused_deps`.
