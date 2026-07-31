# wol

A small Wake-on-LAN web app: a page listing configured devices with a live online/offline status, and a button per device that sends a WoL magic packet. It powers on burst hosts (for example a Proxmox build host that sits at S5 until needed) from a browser.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply the wake targets from your inventory (from the address plan), not from the role. Access control is external — front it with Caddy + Authentik forward-auth; the app itself has no auth.

## Requirements

- Docker host: depends on the `docker` role (Engine + Compose v2).
- Collection `community.docker` (`docker_compose_v2`).
- **Host networking.** The app runs `network_mode: host` so its magic packets broadcast on the target's own L2 segment. Deploy it on a host that sits on the same VLAN as the wake targets (in this estate, the servers VLAN).
- The container needs `NET_RAW` (it drops all other capabilities): the status ping uses a raw ICMP socket, and the binary is `setcap cap_net_raw+ep`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `wol_compose_dir` | `/opt/wol` | Host directory holding the app source, `devices.json`, and `compose.yaml`. |
| `wol_http_port` | `8090` | Web UI port the app listens on (`WOL_LISTEN=0.0.0.0:<port>`); Caddy reverse-proxies to it. |
| `wol_devices` | `[]` | List of `{ name, mac, ip }` wake targets, rendered into `devices.json`. Set from the address plan. |

## Dependencies

`docker` (meta dependency).

## What it does

1. Creates `wol_compose_dir` and copies the Go app source (`app/`) into it.
2. Renders `devices.json` from `wol_devices` and `compose.yaml` from the template.
3. Runs `docker_compose_v2` (`state: present`, `build: policy`) to build and start the container.

Handlers rebuild the image when the source or compose file change (`Rebuild wol`) and restart the container when `devices.json` changes (`Restart wol`).

## The app

A single stdlib-`net/http` Go service (no database, no external dependencies), built multi-stage on `golang`-alpine and shipped on `alpine`, running as a non-root `wol` user:

- `GET /` — the device list with a live status per device (concurrent ICMP echo, 1s timeout).
- `GET /healthz` — container health check (no ping).
- `POST /wake/{name}` — sends the magic packet (6×`0xFF` + MAC×16, broadcast to `255.255.255.255` UDP ports 9 and 7), then redirects to `/`.

Wake targets come only from `devices.json`, so the app redeploys identically with nothing to bootstrap.

## Example

```yaml
- hosts: wol_host
  roles:
    - role: wol
      vars:
        wol_devices:
          - { name: proxmox, mac: "10:ff:e0:84:29:d8", ip: "10.200.20.41" }
        # wol_http_port: 8090   # optional override
```

## Notes

- No authentication in the app by design — put it behind Caddy + Authentik forward-auth.
- Host networking is required for the broadcast to reach the target; a bridged container's broadcast would not leave the container network.
