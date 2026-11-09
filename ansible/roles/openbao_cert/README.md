# openbao_cert

Issues a host's TLS cert from OpenBao PKI and keeps it current. Each run reads the
cert the host already holds and reissues only when it is missing, has fewer than
`openbao_cert_renew_days` left, names a different common name or SANs, or was issued
by a CA other than the one OpenBao serves now (a rebuilt PKI). A re-run with nothing
due changes nothing, and any deploy inside the renewal window renews it.

Part of the `lab` mechanism library: callers pass the cert's name, directory and
names; the deploy supplies OpenBao's address, the PKI role and the SSM parameter.

## How it works

Issuance runs on the controller (`delegate_to: localhost`). It reads the AppRole
(`{"role_id", "secret_id"}` JSON) from SSM with the AWS CLI, logs in once for all the
hosts that need a cert, and issues each host its own. The AppRole secret and the AWS
access never reach the host; only the finished cert and key do. The SSM read, login
and issue run under `no_log`, and the task after each reports a failure's cause
without the secret.

On the host it writes `<dir>/<name>.key` (0600), `<name>.crt` and `<name>-ca.crt`
(OpenBao's CA, for clients on the host to verify against), all owned by
`openbao_cert_owner`/`_group`. With `openbao_cert_metrics_dir` set it also writes
`openbao-cert-<name>.prom` there, holding `tls_cert_not_after_seconds{cert="<name>"}`,
so an alert can see a cert nobody has renewed.

After the role, `openbao_cert_needs_issue` is true on each host that got a new cert
this run, so a caller can reload whatever serves it.

OpenBao's CA is fetched with `validate_certs: false`, as SSSD's CA is: the CA is the
thing later trust is built on, so there is nothing yet to verify it against.

## Requirements

- The AWS CLI on the controller, and `community.crypto` (its `cryptography` comes
  with `ansible-core`).
- An AppRole whose policy may `update` `<mount>/issue/<role>`.

## Role variables

| Variable | Default | Purpose |
|---|---|---|
| `openbao_cert_bao_addr` | `""` | OpenBao's API, e.g. `https://<openbao>:8200`. |
| `openbao_cert_pki_mount` | `pki` | PKI mount to issue from. |
| `openbao_cert_pki_role` | `""` | PKI role to issue from. |
| `openbao_cert_ssm_param` | `""` | SSM parameter holding the AppRole JSON. |
| `openbao_cert_aws_env` | `{}` | Environment for the controller's AWS CLI. |
| `openbao_cert_common_name` | `""` | Common name; always a DNS SAN as well. |
| `openbao_cert_dns_sans` | `[]` | Extra DNS SANs. |
| `openbao_cert_ip_sans` | `[]` | IP SANs. |
| `openbao_cert_ttl` | `8760h` | Lifetime asked for; the PKI role caps it. |
| `openbao_cert_renew_days` | `30` | Reissue once fewer days than this are left. |
| `openbao_cert_name` | `""` | File stem, and the metric's `cert` label. |
| `openbao_cert_dir` | `""` | Directory for the files (created 0750). |
| `openbao_cert_owner` / `_group` | `root` | Owner of the directory and files. |
| `openbao_cert_metrics_dir` | `""` | node_exporter textfile dir; empty writes no metric. |

## Tests

`tests/issue.yml` issues to a throwaway Debian 13 container from a dev OpenBao, with
moto standing in for SSM. It checks the names, key, CA and metric, that a re-run
changes nothing, that a changed SAN, the renewal window and a new CA each reissue,
and that a failed SSM read or a missing setting stops with a clear message and leaves
the cert alone. Needs Docker, the AWS CLI and internet for the images. From
`ansible/`:

```sh
ANSIBLE_ROLES_PATH=roles ansible-playbook -i localhost, roles/openbao_cert/tests/issue.yml
```
