# rpi_radios

Disables the Raspberry Pi's onboard WiFi and Bluetooth at the firmware level to reduce attack surface.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Raspberry Pi wired on Ethernet (the onboard radios must be genuinely unused).
- A writable `config.txt` at `rpi_config_txt`.
- Root via `become`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `rpi_disable_wifi` | `true` | Add the `dtoverlay=disable-wifi` overlay to `config.txt`. |
| `rpi_disable_bt` | `true` | Add the `dtoverlay=disable-bt` overlay to `config.txt`. |
| `rpi_config_txt` | `/boot/firmware/config.txt` | Path to the Pi firmware config file the overlays are written to. |

## Dependencies

None.

## What it does

Adds `dtoverlay=disable-wifi` and `dtoverlay=disable-bt` to `config.txt` when the matching variable is true. Disabling the radios at the firmware level means the interfaces don't appear at all, rather than being brought up and then downed. Every Pi here is hard-wired on eth0, so the onboard radios are unused attack surface. Either change notifies the `Reboot for config.txt` handler.

## Example

```yaml
- hosts: rpi
  roles:
    - role: rpi_radios
```

Disable WiFi only, keep Bluetooth:

```yaml
- hosts: rpi
  roles:
    - role: rpi_radios
      vars:
        rpi_disable_bt: false
```

## Notes

`config.txt` is read only at boot, so the role reboots the host to apply changes. The reboot handler is named `Reboot for config.txt`, the same name used by `rpi_poe_fan`, so a play running both roles reboots the Pi only once.
