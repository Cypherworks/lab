# rpi_poe_fan

Sets quiet PoE HAT fan temperature thresholds in the Raspberry Pi `config.txt`.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Raspberry Pi with the official PoE or PoE+ HAT (firmware-controlled fan).
- A writable `config.txt` at `rpi_config_txt`.
- Root via `become`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `rpi_config_txt` | `/boot/firmware/config.txt` | Path to the Pi firmware config file the thresholds are written to. |
| `rpi_poe_fan_temps` | `{temp0: 70000, temp1: 75000, temp2: 80000, temp3: 82000}` | Fan on/off temperature thresholds in millicelsius, written as `dtparam=poe_fan_<key>=<value>`. |

## Dependencies

None.

## What it does

Writes one `dtparam=poe_fan_tempN=<millicelsius>` line per entry in `rpi_poe_fan_temps` into `config.txt` (existing lines are updated in place). The thresholds are raised well above the idle and light-load temperatures of a DNS Pi, so the fan stays off in normal operation and only spins if the Pi genuinely heats up. Changing a threshold notifies the `Reboot for config.txt` handler.

## Example

```yaml
- hosts: rpi
  roles:
    - role: rpi_poe_fan
```

Raise the thresholds further:

```yaml
- hosts: rpi
  roles:
    - role: rpi_poe_fan
      vars:
        rpi_poe_fan_temps:
          temp0: 72000
          temp1: 77000
          temp2: 82000
          temp3: 84000
```

## Notes

`config.txt` is read only at boot, so the role reboots the host to apply changes. The reboot handler is named `Reboot for config.txt`, the same name used by `rpi_radios`, so a play running both roles reboots the Pi only once.
