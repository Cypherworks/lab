# rpi_poe_fan

Quietens the Raspberry Pi PoE/PoE+ HAT fan by raising its temperature thresholds
in `/boot/firmware/config.txt`, so it stays off under light load and only spins
when the Pi actually heats up. Reboots to apply (config.txt is read at boot).

Apply only to PoE-powered Pis (the DNS Pis here; the kiosk Pi isn't PoE).

## Vars (defaults in `defaults/main.yml`)
- `rpi_poe_fan_temps` — `temp0..temp3` in millicelsius (fan ramp points).
- `rpi_config_txt` — config path (Ubuntu: `/boot/firmware/config.txt`).

Assumes the official PoE/PoE+ HAT (firmware-controlled fan, auto-detected via the
HAT EEPROM). A third-party HAT with its own fan controller won't respond to these.
