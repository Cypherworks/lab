# kiosk

Turns a headless Ubuntu host (a Raspberry Pi here) into a wall-mounted
fullscreen kiosk. `greetd` autostarts `cage` — a minimal Wayland kiosk
compositor — which runs Chromium in `--kiosk` at a single URL, as a dedicated
local user. No desktop environment.

## What it does

1. Installs `greetd`, `cage`, `wlr-randr`, fonts, and Chromium (snap).
2. Creates the `kiosk` user (in `video`/`render`/`input`/`seat`).
3. Renders two launchers: `kiosk-launch` (sets the renderer, execs cage) and
   `kiosk-session` (pins the output mode with `wlr-randr`, execs Chromium).
4. Configures `greetd` to run the launcher on VT1 and masks the console getty —
   greetd owns the seat/VT/logind session, so cage reliably gets the DRM master.

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
