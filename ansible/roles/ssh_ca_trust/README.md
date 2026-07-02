# ssh_ca_trust

Configures sshd to trust an OpenBao SSH CA for user certificates by installing the CA public key and an additive `TrustedUserCAKeys` drop-in.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu host whose `sshd_config` keeps its default `Include /etc/ssh/sshd_config.d/*.conf` line.
- Core `ansible.builtin` modules only: `copy` and `shell`.
- `/usr/sbin/sshd` present for config validation.
- Privilege escalation (`become`) to root.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ssh_ca_public_key` | `""` | The CA public key from OpenBao (`bao read -field=public_key ssh-client-signer/config/ca`). Site data; empty string makes the whole role a no-op. |
| `ssh_ca_trust_file` | `/etc/ssh/openbao_ssh_ca.pub` | Path the CA public key is written to. |
| `ssh_ca_dropin_file` | `/etc/ssh/sshd_config.d/50-openbao-ca.conf` | Path of the sshd drop-in that adds the trust directive. |
| `ssh_ca_sshd_service` | `ssh` | systemd unit for the SSH daemon (Debian/Ubuntu: `ssh`; some distros: `sshd`). |

`ssh_ca_public_key` is the only value that must be supplied. It is a public key, not a secret, but it comes from site infrastructure (OpenBao) rather than the role.

## Dependencies

None.

## What it does

Both tasks are guarded on `ssh_ca_public_key | length > 0`, so an empty key leaves the host untouched.

1. Writes `ssh_ca_public_key` to `ssh_ca_trust_file` (root:root, mode `0644`).
2. Writes an additive drop-in at `ssh_ca_dropin_file` (root:root, mode `0644`) containing a single `TrustedUserCAKeys` directive pointing at the trust file.

Both tasks notify the `Reload sshd` handler, which runs `/usr/sbin/sshd -t && systemctl reload <service>` as one command: validation gates the reload, so a bad config short-circuits, the handler fails loudly, and the running sshd keeps serving its last-good config.

## Example

```yaml
- hosts: ssh_ca_clients
  roles:
    - role: ssh_ca_trust
      vars:
        ssh_ca_public_key: "{{ vault_openbao_ssh_ca_public_key }}"
```

## Notes

The role writes only a `TrustedUserCAKeys` directive. It never touches `PubkeyAuthentication`, `AuthorizedKeysFile`, or `AuthenticationMethods`, so existing key-based logins (the ansible user) are unaffected; certificate trust is purely additive.

Reload, not restart, means live SSH connections are not dropped when trust is applied or updated.

The drop-in depends on the stock `Include /etc/ssh/sshd_config.d/*.conf` line in `sshd_config`. If a hardening layer removes that include, the trust directive will not take effect.
