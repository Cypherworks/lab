# kiosk

Turns a headless Ubuntu host (a Raspberry Pi here) into a wall-mounted
fullscreen kiosk on the long-proven **X11 + Chromium** stack: a tty1 autologin
starts X from the kiosk user's shell profile, and X launches Chromium in
`--kiosk` at a single URL. No desktop environment, no window manager.

## What it does

1. Installs `xserver-xorg`, `xinit`, `x11-xserver-utils`, fonts, and Chromium (snap).
2. Creates the `kiosk` user (in `video`/`render`/`input`/`tty`, real shell).
3. Autologins the kiosk user on tty1 (a `getty@tty1` drop-in).
4. Renders `.bash_profile` (starts X on the tty1 login) and `.xinitrc` (kills
   screen blanking, pins the mode with `xrandr`, execs Chromium fullscreen).

## Required variables

Set either `kiosk_url`, or `kiosk_grafana_url` + `kiosk_grafana_playlist_name`
(the role resolves the Grafana playlist's UID by name and builds the URL).

## Key defaults

| Variable | Default | Purpose |
|----------|---------|---------|
| `kiosk_user` | `kiosk` | Local autologin user. |
| `kiosk_x_output` | `HDMI-1` | X11 output name (Pi 4 HDMI0). |
| `kiosk_resolution` | `1920x1080` | Forced mode; `""` uses the monitor default. |
| `kiosk_chromium_extra_flags` | `[]` | Extra Chromium flags. |

## Notes

The output name (`kiosk_x_output`) may need tuning to the monitor — check
`xrandr` on the box if the screen is blank or wrong-sized. Runs as a local
autologin user, independent of any SSSD/LDAP login on the host.
