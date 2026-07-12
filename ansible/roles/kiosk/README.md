# kiosk

Turns a headless Ubuntu host (a Raspberry Pi here) into a wall-mounted
fullscreen kiosk on the long-proven **X11 + Chromium** stack, run as a systemd
service (`kiosk.service`) so it's managed with `systemctl start/stop/restart/
status kiosk`. The service `startx`s on VT1 and X launches Chromium in `--kiosk`
at a single URL. No desktop environment, no window manager.

## What it does

1. Installs `xserver-xorg`, `xinit`, `x11-xserver-utils`, fonts, and Chromium (snap).
2. Creates the `kiosk` user (in `video`/`render`/`input`/`tty`).
3. Renders `.xinitrc` (kills screen blanking, pins the mode with `xrandr`, execs
   Chromium fullscreen) and the Xorg wrapper so the service can start X.
4. Installs + enables `kiosk.service` on VT1 (masks the console getty), and an
   optional daily restart timer (`kiosk_daily_restart`) that clears memory creep.

## Required variables

Set either `kiosk_url`, or `kiosk_grafana_url` + `kiosk_grafana_playlist_name`
(the role resolves the Grafana playlist's UID by name and builds the URL).

## Key defaults

| Variable | Default | Purpose |
|----------|---------|---------|
| `kiosk_user` | `kiosk` | Local user the kiosk runs as. |
| `kiosk_x_output` | `HDMI-1` | X11 output name (Pi 4 HDMI0). |
| `kiosk_resolution` | `1920x1080` | Forced mode; `""` uses the monitor default. |
| `kiosk_chromium_extra_flags` | `[]` | Extra Chromium flags. |

## Notes

The output name (`kiosk_x_output`) may need tuning to the monitor — check
`xrandr` on the box if the screen is blank or wrong-sized. Runs as a local
autologin user, independent of any SSSD/LDAP login on the host.
