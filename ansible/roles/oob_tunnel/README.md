# oob_tunnel

An out-of-band, break-glass access path that does not depend on the primary remote-access
overlay. A client host dials a persistent reverse SSH tunnel (via `autossh`) outbound to a
relay's sshd; the relay exposes that tunnel on its own loopback only. An operator who is
already authenticated to the relay can then hop back through the tunnel to the client and,
from there, into the rest of the network.

Part of the `lab` mechanism library: a generic, parameterised role. Supply the relay
address, ports and key material from your inventory and SOPS, not from the role.

## Why it survives an overlay outage

The path is plain outbound SSH to an existing sshd, so it needs no inbound firewall change
and shares nothing with the overlay it backstops — no VPN control plane, no subnet routers.
When the overlay fails (including when a hardening run takes the overlay routers down), this
path is unaffected. On the relay, a scoped `Match User` block keeps forwarding enabled for
the tunnel user even if a later global `AllowTcpForwarding no` is applied, so hardening the
relay can't silently sever the break-glass path.

## Security properties

- The relay account is forwarding-only: `nologin` shell, and an authorized_keys entry
  restricted to `restrict,port-forwarding` (no shell, pty, agent or command).
- With the relay's default `GatewayPorts no`, the reverse forward binds the relay's
  loopback, so reaching the client requires first authenticating to the relay — two auth
  hops, not one.
- The tunnel private key lives only on the client (0600, from SOPS); the relay holds just
  the public half.

## Modes

Set `oob_tunnel_mode`:

- `client` — installs `autossh`, the tunnel key and a systemd unit that maintains the
  reverse tunnel with SSH keepalives and automatic reconnect.
- `relay` — provisions the forwarding-only user and authorises the tunnel key.

## Requirements

- Debian/Ubuntu with systemd on both ends; `autossh` available in apt (client).
- `ansible.posix` (relay, for `authorized_key`).
- A dedicated SSH keypair for the tunnel (generate out of band; private half in SOPS).

## Role variables

| Variable | Default | Purpose |
|---|---|---|
| `oob_tunnel_mode` | `""` | `client` or `relay`. |
| `oob_tunnel_relay_host` | `""` | Public host the client dials. |
| `oob_tunnel_relay_ssh_port` | `22` | The relay's sshd port. |
| `oob_tunnel_relay_user` | `oob` | Forwarding-only account on the relay. |
| `oob_tunnel_remote_bind_port` | `2222` | Relay-loopback port mapped to the client's SSH. |
| `oob_tunnel_local_ssh_port` | `22` | Client port the tunnel exposes. |
| `oob_tunnel_config_dir` | `/etc/oob-tunnel` | Client config/key directory (0700). |
| `oob_tunnel_private_key` | `""` | Tunnel private key (client, from SOPS). |
| `oob_tunnel_public_key` | `""` | Tunnel public key (relay). |
| `oob_tunnel_relay_host_key` | `""` | Optional relay host-key pin; empty = accept-new (TOFU). |

## Using the tunnel (break-glass)

From the operator's machine, once the tunnel is up:

```
ssh -p <relay_ssh_port> <admin>@<relay_host>          # into the relay
ssh -p <remote_bind_port> <user>@127.0.0.1            # through the tunnel onto the client
```

From the client you can then reach the rest of the network as that host normally would.
