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

## Keys

Two keys, and only one of them is new:

- The **tunnel machine key** (private half on the client, public half on the relay). The
  client dials the relay unattended, so it needs its own credential — but it is
  forwarding-only on the relay and authenticates no one to the client, so it is
  deliberately low-value. No private key is ever placed on the relay.
- The **operator's own key**, unchanged. You reach the client with the key you already
  use, via `ProxyJump` — see below. There is no per-operator key to distribute.

## Using the tunnel (break-glass)

Chain through the relay with your existing key using `ProxyJump`, so your key
authenticates both hops from your own machine and never touches the relay (unlike agent
forwarding, which exposes your agent on the relay):

```
ssh -J <admin>@<relay_host>:<relay_ssh_port> -p <remote_bind_port> <you>@127.0.0.1
```

Or as an SSH config entry:

```
Host oob-client
    HostName 127.0.0.1
    Port <remote_bind_port>
    ProxyJump <admin>@<relay_host>:<relay_ssh_port>
    User <you>
```

The relay only ever forwards ciphertext of your client session — it terminates the outer
tunnel transport but cannot read the inner SSH session. From the client you can then reach
the rest of the network as that host normally would.

## Hardening notes

- Pin the relay host key (`oob_tunnel_relay_host_key`) to remove the first-connect MITM
  window; empty falls back to trust-on-first-use.
- The relay authorises the tunnel key with `permitopen="none"` and a scoped `permitlisten`,
  so a stolen tunnel key can only create the one intended forward.
