# tailscale

Installs Tailscale and joins the host to a Headscale/Tailscale overlay using a pre-auth key.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Ubuntu host (the apt repo is configured against `pkgs.tailscale.com/stable/ubuntu`).
- Core `ansible.builtin` modules only: `deb822_repository`, `apt`, `systemd_service`, `command`, `set_fact`.
- Privilege escalation (`become`) to root.
- A reachable Headscale/Tailscale control server and a valid pre-auth key.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `tailscale_login_server` | `https://headscale.example.com` | Control-server URL. The default is a placeholder; set your control server. |
| `tailscale_authkey` | `""` | Short-lived pre-auth key minted on Headscale. **Secret — from SOPS**; never defaulted to a real value. |
| `tailscale_authkey_command` | `""` | Optional command run at join time to mint a key, used instead of `tailscale_authkey`. For a host that is itself the control server and can issue its own keys. Its last stdout line is taken as the key. |
| `tailscale_accept_routes` | `true` | Whether to accept the lab supernet advertised by the HA subnet routers (`--accept-routes`). |
| `tailscale_hostname` | `"{{ inventory_hostname }}"` | Hostname registered on the tailnet. |
| `tailscale_up_timeout` | `"30s"` | `--timeout` for `tailscale up`, so an unreachable control plane fails the task instead of blocking the run. |
| `tailscale_metrics` | `false` | Install the peer collector (`tasks_from: metrics`). `false` removes it and its count. |
| `tailscale_metrics_dir` | `/var/lib/prometheus/node-exporter` | node_exporter's textfile directory, where the collector writes `tailscale.prom`. |
| `tailscale_metrics_interval` | `1min` | How often the collector counts. |

Supply the key from SOPS via `tailscale_authkey`, or from `tailscale_authkey_command` for a host that mints its own (leave the other empty). `tailscale_login_server` defaults to the placeholder `https://headscale.example.com` and must be overridden.

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

`tasks/metrics.yml` is separate from the join, for hosts whose Tailscale is installed some other way (the subnet routers, by cloud-init). With `tailscale_metrics` on, a timer runs `/usr/local/sbin/tailscale-metrics`, which reads `tailscale status --json` and writes `tailscale_peers_active` (peers this node sent a packet to in the last two minutes) and `tailscale_peers_active_not_direct` (those with no direct address, so through DERP or nowhere) to `tailscale.prom`. Idle peers have no address either, which is why only active ones count. On any failure, a stopped `tailscaled` included, it removes the file rather than leave an old count, and the unit fails. It runs as root with no capabilities and no network, writing only that directory. `tests/metrics.yml` runs it on Debian 13 under systemd, joined to a throwaway Headscale with one peer it can reach and one it can't.
