# kiosk

Turns a headless Ubuntu host (a Raspberry Pi here) into a wall-mounted
fullscreen kiosk. `cage` — a minimal Wayland kiosk compositor — runs Chromium
in `--kiosk` at a single URL, auto-started on boot as a dedicated local user via
a systemd service. No desktop environment.

## What it does

1. Installs `cage`, `wlr-randr`, fonts, and Chromium (snap).
2. Creates the `kiosk` user (in `video`/`render`/`input`).
3. Renders a session launcher (`/usr/local/bin/kiosk-session`) that pins the
   output mode with `wlr-randr` then `exec`s Chromium.
4. Renders + enables `kiosk.service` — a PAM/logind session on tty1, so cage
   gets a seat and the DRM/input devices without a separate seatd. Restarts on
   crash.

## Required variables

| Variable | Purpose |
|----------|---------|
| `kiosk_url` | The fullscreen URL to display. |

## Key defaults

| Variable | Default | Purpose |
|----------|---------|---------|
| `kiosk_user` | `kiosk` | Local user the session runs as. |
| `kiosk_output` | `HDMI-A-1` | Wayland output name (Pi 4 HDMI0). |
| `kiosk_resolution` | `1920x1080` | Forced output mode; `""` uses the EDID default. |
| `kiosk_rotate` | `""` | `90`/`180`/`270` for a rotated screen. |
| `kiosk_chromium_extra_flags` | `[]` | Extra Chromium flags. |

## Notes

The output name (`kiosk_output`) and mode may need tuning to the actual monitor
— check `wlr-randr` output on the box if the screen is blank or wrong-sized.
Runs as a local user, independent of any SSSD/LDAP login on the host.
