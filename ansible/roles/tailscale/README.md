# tailscale

Installs Tailscale and joins the host to a Headscale/Tailscale overlay using a pre-auth key.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Ubuntu host (the apt repo is configured against `pkgs.tailscale.com/stable/ubuntu`).
- Core `ansible.builtin` modules only: `deb822_repository`, `apt`, `systemd_service`, `command`.
- Privilege escalation (`become`) to root.
- A reachable Headscale/Tailscale control server and a valid pre-auth key.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `tailscale_login_server` | `https://headscale.example.com` | Control-server URL. Site-specific default; adopters override it. |
| `tailscale_authkey` | `""` | Short-lived pre-auth key minted on Headscale. **Secret — from SOPS**; never defaulted to a real value. |
| `tailscale_authkey_command` | `""` | Optional command run at join time to mint a key, used instead of `tailscale_authkey`. For a host that is itself the control server and can issue its own keys. Its last stdout line is taken as the key. |
| `tailscale_accept_routes` | `true` | Whether to accept the lab supernet advertised by the HA subnet routers (`--accept-routes`). |
| `tailscale_hostname` | `"{{ inventory_hostname }}"` | Hostname registered on the tailnet. |
| `tailscale_up_timeout` | `"30s"` | `--timeout` for `tailscale up`, so an unreachable control plane fails the task instead of blocking the run. |

Supply the key from SOPS via `tailscale_authkey`, or from `tailscale_authkey_command` for a host that mints its own (leave the other empty). `tailscale_login_server` defaults to the Cypherworks control server and should be overridden for any other deployment.

## Dependencies

None.

## What it does

1. Adds the Tailscale apt repository as a deb822 source named `tailscale`, keyed on `ansible_distribution_release`.
2. Installs the `tailscale` package via apt.
3. Enables and starts `tailscaled`.
4. Reads the current backend state with `tailscale status --json` (`changed_when: false`, failures tolerated) and sets `tailscale_needs_join` (true when `status` failed or `BackendState != Running`).
5. When a join is needed and `tailscale_authkey_command` is set, runs it to mint a key; otherwise uses `tailscale_authkey`.
6. Runs `tailscale up` with the login server, the selected key, `--timeout {{ tailscale_up_timeout }}`, optional `--accept-routes`, and hostname — only when `tailscale_needs_join`. The mint and join tasks use `no_log: true` to keep the key out of logs.

## Example

```yaml
- hosts: overlay_nodes
  roles:
    - role: tailscale
      vars:
        tailscale_login_server: https://headscale.example.com
        tailscale_authkey: "{{ vault_tailscale_authkey }}"
```

## Notes

`tailscale up` runs only when the node is not already `Running`, so re-runs do not churn an established connection. Applying state directly is why the role ships no active handlers.

The pre-auth key is short-lived. Supply it per host from SOPS (`tailscale_authkey`), or have the host mint its own at join time (`tailscale_authkey_command`) when it is the control server. Because the mint and join tasks carry `no_log: true`, the key is redacted from Ansible output.
