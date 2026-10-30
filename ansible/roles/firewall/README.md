# firewall

A host firewall with ufw: inbound denied by default, SSH left open so Ansible keeps its way in, and whatever else a host serves listed in its variables. Run it before a role that starts a service, so nothing listens unprotected.

Part of the `lab` mechanism library: a generic, parameterised role. Supply the rules from your inventory, not from the role.

## How it works

All rules go in before ufw is enabled, so default-deny can't cut the connection that's applying them. SSH is held apart from `firewall_rules`, so setting those can't drop it by accident. If `firewall_ssh_sources` is set, the role refuses to run when this run's own SSH source isn't in it. A run that isn't over SSH, such as one against a container, skips that check.

Rules are additive: the role adds what's listed and never removes a rule, so dropping one from the list, or narrowing `firewall_ssh_sources`, leaves the old rule in place until you delete it with `ufw delete`.

`tests/firewall.yml` runs ufw inside a throwaway container and tries each port with a real connection from clients at fixed addresses. It needs Docker, `community.docker`, and internet for its apt packages.

## Requirements

- Ubuntu or Debian with `ufw` in apt.
- `community.general`.

## Role variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `firewall_enabled` | `true` | `false` writes the rules without turning ufw on. |
| `firewall_default_incoming` | `deny` | Default inbound policy. |
| `firewall_default_outgoing` | `allow` | Default outbound policy. |
| `firewall_allow_loopback` | `true` | Allow traffic on `lo`. |
| `firewall_ssh_port` | `22` | SSH port. |
| `firewall_ssh_sources` | `[]` | Who may SSH in; empty is anywhere. |
| `firewall_rules` | `[]` | The host's other rules: `port`, and optionally `proto`, `from`, `rule` and `comment`. |
