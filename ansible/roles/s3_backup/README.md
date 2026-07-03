# s3_backup

Generic scheduled backup to S3: a oneshot systemd service and timer that run a
caller-supplied dump command and upload the artifact to a bucket with the AWS CLI.

Part of the `lab` mechanism library: a generic, parameterised role. Supply site data
(bucket, schedule, dump command, credentials) from your inventory and SOPS, not from the
role. The role holds no site-specific values.

Retention is not managed here — it belongs on the bucket's lifecycle policy (Terraform),
so backups expire regardless of the producer's state. Every unit, script and working path
is namespaced by `s3_backup_name`, so several backups can run on one host without
colliding.

## Requirements

- Debian/Ubuntu with `apt` and systemd.
- Outbound network to the S3 endpoint.
- A bucket and a scoped IAM user (PutObject) provisioned out of band (e.g. Terraform).

## How it works

A daily timer runs `/usr/local/bin/s3-backup-<name>.sh`, which sources the S3 credentials
and command environment, exports `$BACKUP_FILE` (a timestamped path), runs
`s3_backup_command`, and uploads the result. A command that produces no file (an empty
`$BACKUP_FILE`) is treated as a clean no-op — so a node can opt out, e.g. a database
replica that only backs up when it is the leader.

## Role variables

| Variable | Default | Purpose |
|---|---|---|
| `s3_backup_enabled` | `false` | Master switch; the role no-ops when false. |
| `s3_backup_name` | `""` | Identifier; namespaces the units, script and working dir. |
| `s3_backup_bucket` | `""` | Target S3 bucket. |
| `s3_backup_schedule` | `*-*-* 03:30:00` | systemd `OnCalendar` expression. |
| `s3_backup_dir` | `/var/lib/s3-backup/<name>` | Working directory (mode 0700). |
| `s3_backup_extension` | `bak` | Artifact filename suffix. |
| `s3_backup_command` | `""` | Dump command; writes its artifact to `$BACKUP_FILE`. |
| `s3_backup_packages` | `[]` | Extra apt packages the command needs. |
| `s3_backup_env` | `{}` | Extra environment (secrets) for the command, written 0600. |
| `s3_backup_aws_access_key_id` | `""` | Scoped uploader access key (from SOPS). |
| `s3_backup_aws_secret_access_key` | `""` | Scoped uploader secret key (from SOPS). |
| `s3_backup_aws_region` | `""` | Bucket region. |

## Example

```yaml
- hosts: vaultwarden
  roles:
    - role: s3_backup
      when: s3_backup_enabled | bool
  vars:
    s3_backup_enabled: true
    s3_backup_name: vaultwarden
    s3_backup_bucket: example-vaultwarden-backups
    s3_backup_extension: sqlite3
    s3_backup_packages: [sqlite3]
    s3_backup_command: >-
      sqlite3 "/var/lib/docker/volumes/vaultwarden_vaultwardendata/_data/db.sqlite3"
      ".backup '${BACKUP_FILE}'"
    s3_backup_aws_access_key_id: "{{ vault_backup_key_id }}"
    s3_backup_aws_secret_access_key: "{{ vault_backup_secret }}"
    s3_backup_aws_region: eu-west-2
```
