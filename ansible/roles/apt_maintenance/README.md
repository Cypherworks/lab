# apt_maintenance

Weekly apt upgrade from every origin, then a reboot, in a slot of the host's own, gated on checks that the rest of the estate is healthy. `unattended_upgrades` stays the nightly security floor, which keeps working when nothing else does; this role applies everything else (`-updates` and any configured third-party repo) and does the reboots that role leaves off.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (slots, gate checks) from your inventory, not from the role.

Nothing runs centrally and no credential is stored: each host upgrades itself on its timer. Staggering comes from giving each host its own slot, and the gate replaces `serial: 1`. A host whose gate fails defers and touches nothing, so a host that didn't come back from its reboot keeps every later host whose gate checks it from rebooting too. The gate is only as strong as its checks: make them test the service, not just the process.

## Requirements

- Debian or Ubuntu with systemd. Tested on Ubuntu 24.04 and Debian 13.
- Core `ansible.builtin` modules only.
- Privilege escalation (`become`) to root.
- node_exporter's textfile collector reading `apt_maintenance_textfile_dir`, for the metrics.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `apt_maintenance_on_calendar` | `""` | **Required.** This host's systemd `OnCalendar` slot, e.g. `Sun *-*-* 02:00:00`. Checked with `systemd-analyze calendar`. |
| `apt_maintenance_gate_checks` | `[]` | Shell commands, one line each, that must all exit 0 before an upgrade and again after its reboot. Run as root through `sh -c` with a 60s timeout each. A failing check is logged with its last output line, so keep secrets out of them. |
| `apt_maintenance_boot_confirm_timeout` | `1800` | Seconds the gate may take to pass after the reboot before the run counts as failed. |
| `apt_maintenance_boot_confirm_interval` | `30` | Seconds between gate attempts after the reboot. |
| `apt_maintenance_reboot_deadline` | `1200` | Seconds after the slot starts past which the run won't reboot. Keep it below the gap to the next slot that shares this host's gate, less a reboot, so a slow upgrade can't overlap the next host's. |
| `apt_maintenance_lock_timeout` | `600` | Seconds apt waits for the dpkg lock (`DPkg::Lock::Timeout`), e.g. while unattended-upgrades runs. |
| `apt_maintenance_update_retry_delay` | `60` | Seconds between the three `apt-get update` attempts. |
| `apt_maintenance_textfile_dir` | `/var/lib/prometheus/node-exporter` | Where `apt_maintenance.prom` is written; the same directory the collectors' `apt.prom` uses. |
| `apt_maintenance_reboot_command` | `systemctl reboot` | How the host reboots. In an Incus container this restarts the container. |

## What it does

1. Refuses a host with no slot, or with a gate check that spans lines.
2. Installs `/usr/local/sbin/apt-maintenance`, its settings and gate in `/etc/apt-maintenance/` (`0600`), and a state directory `/var/lib/apt-maintenance/` (`0700`).
3. Installs and enables `apt-maintenance.timer` (the slot, without `Persistent=`, so a slot missed while the host was down is skipped rather than run at boot, which would line hosts up after a power cut), `apt-maintenance.service` (`apt-maintenance run`), and `apt-maintenance-confirm.service`, which runs at boot only after a maintenance reboot.

Each run:

1. Runs the gate. If any check fails, records `deferred` and stops.
2. `apt-get update --error-on=any` (three tries), then `apt-get upgrade --with-new-pkgs`, non-interactive, keeping local conffiles. A repo that can't be fetched fails the run, so a broken source never reads as patched. New dependencies such as a new kernel come in; nothing is removed, holds stay held, and there is no autoremove (lab-deploy#194). A failure records `failed`, fails the unit and doesn't reboot.
3. If no package changed and `/run/reboot-required` is absent, records success.
4. Otherwise, before rebooting: past `apt_maintenance_reboot_deadline` it records `failed` and stays up, since the next host's slot may already have started; it runs the gate again and, if that now fails, records `deferred` and stays up. Only then does it record `rebooting` and reboot.
5. After the reboot, the confirm unit retries the gate until it passes (success) or `apt_maintenance_boot_confirm_timeout` runs out (`failed`, unit failed).

A host where unattended-upgrades applied a library fix during the week, with nothing new by the slot and no reboot flag, isn't rebooted, so long-running services keep the old library until the next slot that changes something.

## Metrics

`apt_maintenance.prom`, written atomically:

| Series | Meaning |
|--------|---------|
| `apt_maintenance_last_run_timestamp_seconds` | When the last run started. |
| `apt_maintenance_last_success_timestamp_seconds` | When a run last left the host patched and its gate passing. Absent until the first success. |
| `apt_maintenance_result{result}` | 1 on the current outcome: `ok`, `deferred`, `failed` or `rebooting`. |

A missed or dead run shows as `time() - apt_maintenance_last_success_timestamp_seconds` growing past a week, or the series missing.

## Example

```yaml
- hosts: web
  become: true
  roles:
    - role: apt_maintenance
      vars:
        apt_maintenance_on_calendar: "{{ maintenance_slots[inventory_hostname] }}"
        apt_maintenance_gate_checks:
          - "timeout 5 bash -c '</dev/tcp/web-peer.example/443'"
```

## Tests

`tests/maintenance.yml` runs the units in a systemd container against a local apt repo, on each OS:

```bash
cd ansible
ANSIBLE_ROLES_PATH=roles ansible-playbook -i localhost, -c local roles/apt_maintenance/tests/maintenance.yml
ANSIBLE_ROLES_PATH=roles ansible-playbook -i localhost, -c local roles/apt_maintenance/tests/maintenance.yml -e _image=debian:13
```
