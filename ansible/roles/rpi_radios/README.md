# rpi_radios

Disables the Raspberry Pi's onboard WiFi and Bluetooth via `config.txt` overlays
(`disable-wifi`, `disable-bt`) — the interfaces don't come up at all. For
hard-wired Pis where the radios are just attack surface. Reboots to apply.

## Vars (defaults in `defaults/main.yml`)
- `rpi_disable_wifi` (true), `rpi_disable_bt` (true)
- `rpi_config_txt` — config path (Ubuntu: `/boot/firmware/config.txt`)

Shares the `Reboot for config.txt` handler name with `rpi_poe_fan`, so a play
applying both reboots once.
