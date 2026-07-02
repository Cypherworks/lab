# openbao

Installs a single-node, integrated-raft OpenBao secrets manager with AWS-KMS auto-unseal, and reconciles its PKI, listener cert, OIDC login, SSH CA, and raft snapshots.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- A Debian-family host (installs from the upstream `.deb`); an unprivileged LXC container is the intended target.
- AWS KMS key for auto-unseal, plus AWS credentials for the seal and (optionally) the snapshot uploader, supplied from SOPS.
- `bao operator init` run once by hand after first start (see What it does); the resulting root token supplied back as `openbao_root_token` for the reconcile steps.
- Caddy (or equivalent) terminating TLS in front of the listener and serving the PKI issuing/CRL URLs.
- For snapshots: an S3 bucket and a scoped `openbao-snapshot` IAM user.

## Role variables

### Core

| Variable | Default | Description |
| --- | --- | --- |
| `openbao_version` | `2.5.5` | Pinned upstream release. |
| `openbao_arch` | `amd64` | Package architecture. |
| `openbao_release_base` | `https://github.com/openbao/openbao/releases/download/v{{ openbao_version }}` | Release download base. |
| `openbao_deb_url` | `{{ openbao_release_base }}/openbao_{{ openbao_version }}_linux_{{ openbao_arch }}.deb` | Full `.deb` URL. |
| `openbao_data_dir` | `/opt/openbao/data` | Raft data directory. |
| `openbao_tls_dir` | `/opt/openbao/tls` | Listener cert/key directory. |
| `openbao_tls_cert_file` | `{{ openbao_tls_dir }}/tls.crt` | Listener cert path (overwritten in place by the PKI-issued cert). |
| `openbao_tls_key_file` | `{{ openbao_tls_dir }}/tls.key` | Listener key path. |
| `openbao_listen_address` | `0.0.0.0:8200` | Listener bind; pin to the instance's own address in the deploy. |
| `openbao_api_addr` | `""` | Advertised API address, e.g. `https://<instance-ip>:8200` (site data). |
| `openbao_cluster_addr` | `""` | Advertised raft cluster address, e.g. `https://<instance-ip>:8201` (site data). |
| `openbao_node_id` | `{{ inventory_hostname }}` | Raft node id. |
| `openbao_ui` | `true` | Enable the web UI. |
| `openbao_disable_mlock` | `true` | Disable mlock (LXC lacks `CAP_IPC_LOCK`); the unit sets `MemorySwapMax=0` instead. |
| `openbao_kms_key_id` | `""` | AWS KMS key id for auto-unseal (not secret; lives in the config). |
| `openbao_aws_region` | `""` | AWS region for the KMS seal. |
| `openbao_aws_access_key_id` | `""` | AWS access key for the seal (secret; via EnvironmentFile, from SOPS). |
| `openbao_aws_secret_access_key` | `""` | AWS secret key for the seal (secret; from SOPS). |

### PKI

The PKI reconcile runs only when `openbao_root_token` is set. The root CA is generated exactly once (guarded on an existing CA).

| Variable | Default | Description |
| --- | --- | --- |
| `openbao_root_token` | `""` | Root token from SOPS; empty skips the entire PKI/OIDC/SSH/snapshot reconcile. |
| `openbao_pki_mount` | `pki` | PKI secrets engine mount path. |
| `openbao_pki_max_lease_ttl` | `87600h` | PKI max lease TTL (10y ceiling). |
| `openbao_pki_root_ttl` | `87600h` | Root CA TTL. |
| `openbao_pki_root_common_name` | `""` | Root CA common name (site data). |
| `openbao_pki_issuing_url` | `""` | Where clients fetch the CA (served via Caddy; site data). |
| `openbao_pki_crl_url` | `""` | CRL distribution point (site data). |
| `openbao_pki_roles` | `[]` | Issuing roles, each `{name, allowed_domains, allow_subdomains, max_ttl}`. |

### Listener cert

| Variable | Default | Description |
| --- | --- | --- |
| `openbao_listener_cert_role` | `""` | Issuing role for the listener cert; empty leaves the bootstrap self-signed cert. |
| `openbao_listener_cert_ttl` | `2160h` | Listener cert TTL (90d). |
| `openbao_listener_common_name` | `""` | Listener cert CN (the address Caddy dials; site data). |
| `openbao_listener_ip_sans` | `""` | Listener cert IP SANs (site data). |

### OIDC (human login via Authentik)

| Variable | Default | Description |
| --- | --- | --- |
| `openbao_oidc_enabled` | `false` | Enable `bao login -method=oidc`; machine auth stays on AppRole/token. |
| `openbao_oidc_discovery_url` | `""` | Authentik OIDC discovery URL. |
| `openbao_oidc_client_id` | `""` | OIDC client id. |
| `openbao_oidc_client_secret` | `""` | OIDC client secret (from SOPS). |
| `openbao_oidc_default_role` | `default` | Default OIDC login role. |
| `openbao_oidc_roles` | `[]` | Login roles, each `{name, token_policies, allowed_redirect_uris}` plus optional `user_claim`/`groups_claim`/`oidc_scopes`/`ttl`. |
| `openbao_policies` | `[]` | Named ACL policies to manage, each `{name, rules}`. |

### SSH CA (audited short-lived SSH)

The CA is generated exactly once (guarded); regenerating it would invalidate every host's trust.

| Variable | Default | Description |
| --- | --- | --- |
| `openbao_ssh_enabled` | `false` | Enable the SSH CA. |
| `openbao_ssh_mount` | `ssh-client-signer` | SSH secrets engine mount path. |
| `openbao_ssh_default_extensions` | `{permit-pty: ""}` | Default cert extensions (a map field). |
| `openbao_ssh_allowed_extensions` | `permit-pty,permit-port-forwarding` | Allowed cert extensions. |
| `openbao_ssh_roles` | `[]` | Signing roles, each `{name, allowed_users}` plus optional `ttl`/`default_extensions`/`allowed_extensions`; set `principals_from_oidc: true` to lock the principal to the caller's OIDC identity. |

### Snapshots (raft DR backups to S3)

| Variable | Default | Description |
| --- | --- | --- |
| `openbao_snapshot_enabled` | `false` | Enable daily raft snapshots; empty bucket also disables. |
| `openbao_snapshot_bucket` | `""` | S3 bucket for snapshots (site data). |
| `openbao_snapshot_schedule` | `*-*-* 03:00:00` | systemd `OnCalendar` schedule (daily 03:00). |
| `openbao_snapshot_dir` | `/var/lib/openbao-snapshots` | Snapshot working directory. |
| `openbao_snapshot_approle` | `snapshot` | AppRole role name for the snapshot job. |
| `openbao_snapshot_policy_name` | `snapshot` | Snapshot-only ACL policy name. |
| `openbao_snapshot_aws_access_key_id` | `""` | S3 uploader access key (scoped IAM user; from SOPS). |
| `openbao_snapshot_aws_secret_access_key` | `""` | S3 uploader secret key (from SOPS). |
| `openbao_snapshot_aws_region` | `""` | S3 uploader region. |

## Dependencies

None (no `meta/main.yml`). The reconcile steps call the `bao` CLI shipped by the package. Snapshots install `awscli` from apt.

## What it does

1. Installs the pinned OpenBao `.deb`; the package creates the `openbao` user, the systemd unit, `/etc/openbao/`, and a self-signed bootstrap TLS cert.
2. Renders the auto-unseal credentials as the systemd `EnvironmentFile` (`/etc/openbao/openbao.env`) and the config (`/etc/openbao/openbao.hcl`) — AWS creds never touch the config file.
3. Enables and starts the service. It comes up **sealed and uninitialised**: run `bao operator init` once by hand to emit the recovery keys and root token, capture them into the break-glass kit and SOPS. After that, the KMS seal auto-unseals on every restart.
4. With `openbao_root_token` supplied, reconciles the PKI (mount, tune, root CA once, URLs, issuing roles).
5. Optionally swaps the bootstrap listener cert for one issued by the internal CA (guarded on a `.ca-issued` marker), so Caddy can verify the upstream against the CA.
6. Optionally configures OIDC login (ACL policies, auth method, config, roles).
7. Optionally configures the SSH CA (engine, CA keypair once, signing roles; `principals_from_oidc` roles look up the OIDC accessor).
8. Optionally configures daily raft snapshots: a snapshot-only AppRole (secret_id generated once, guarded on its creds file), the uploader creds, the snapshot script, and a systemd service + timer.

## Example

```yaml
- hosts: openbao
  become: true
  roles:
    - role: openbao
      vars:
        openbao_api_addr: "https://10.30.0.10:8200"
        openbao_cluster_addr: "https://10.30.0.10:8201"
        openbao_kms_key_id: "{{ vault_openbao_kms_key_id }}"
        openbao_aws_region: eu-west-2
        openbao_aws_access_key_id: "{{ vault_openbao_seal_access_key }}"
        openbao_aws_secret_access_key: "{{ vault_openbao_seal_secret_key }}"
        openbao_root_token: "{{ vault_openbao_root_token }}"
        openbao_pki_root_common_name: "Lab Root CA"
        openbao_pki_issuing_url: "https://pki.example.com/ca"
        openbao_pki_crl_url: "https://pki.example.com/crl"
        openbao_pki_roles:
          - name: lab
            allowed_domains: "example.com"
            allow_subdomains: true
            max_ttl: "720h"
        openbao_listener_cert_role: lab
        openbao_listener_common_name: openbao.example.com
        openbao_listener_ip_sans: "10.30.0.10"
```

## Notes

- The whole PKI/listener-cert/OIDC/SSH/snapshot reconcile is gated on `openbao_root_token`. Leave it empty until after the manual `bao operator init`.
- The root CA and the SSH CA are generated exactly once and never regenerated (guarded), so re-runs and post-restore runs preserve trust continuity across a rebuild. Regenerating either would invalidate all existing trust.
- The listener cert is issued once and guarded on `.ca-issued`; delete the marker (or the future renewal timer) to rotate before expiry.
- The snapshot AppRole `secret_id` is generated once and guarded on its creds file, so re-runs don't rotate it.
- `disable_mlock` is set because the unprivileged LXC container can't grant `CAP_IPC_LOCK`; the unit's `MemorySwapMax=0` keeps secrets off swap.
- `Restart openbao` re-reads config and (once initialised) auto-unseals via KMS — no manual unseal needed.
