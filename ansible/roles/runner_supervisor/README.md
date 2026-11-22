# runner_supervisor

Runs lab's `automation/cmd/runner-supervisor` as one systemd instance per
Incus VM (`runner-supervisor@<vm>`). Each instance restores its VM to the
newest `golden-*` snapshot, mints a single-job JIT config with the GitHub
token, hands it in with `start-jit-runner` (the `github_jit_runner` role),
and resets the VM once the job ends. The VM never sees the token. The
design is lab-deploy's `docs/docker-runner-reset.md`.

## Requirements

Debian 13 or Ubuntu 24.04 with systemd, as root (or `become`). On the
control node: Go (the version `automation/go.mod` asks for, or newer) and
the lab checkout the roles come from, since the role builds the binary from
it. `community.crypto` for the client certificate. Internet access to
`pkgs.zabbly.com` and the GitHub API.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `runner_supervisor_vms` | `[]` | The Incus VMs to supervise, one instance each. Required. |
| `runner_supervisor_org` | `""` | The GitHub org the runners join. Required. |
| `runner_supervisor_labels` | `[self-hosted, docker]` | Runner labels. GitHub doesn't add `self-hosted` to a JIT runner, so it's listed. |
| `runner_supervisor_remote_name` | `lab` | Name of the incus remote the supervisor uses. |
| `runner_supervisor_remote_address` | `""` | Host or IP of the Incus API (port 8443). Required. |
| `runner_supervisor_server_cert` | `""` | The cluster's server certificate (PEM), pinned so the remote isn't trusted on first use. Required. |
| `runner_supervisor_project` | `""` | The Incus project holding the VMs. Required. |
| `runner_supervisor_token` | `""` | GitHub token that mints, polls and deletes runners, from SOPS. Required. |
| `runner_supervisor_github_url` | `https://api.github.com` | GitHub REST API base URL. |
| `runner_supervisor_metrics_dir` | `""` | node_exporter's textfile directory. Empty writes no metrics. |
| `runner_supervisor_cert_dest` | `""` | Path on the control node the client certificate is fetched to, for the deploy to trust (Terraform `incus_certificate`, restricted to the project). Required. |
| `runner_supervisor_cert_name` | `runner-supervisor` | The client certificate's common name. |
| `runner_supervisor_stop_timeout` | `70min` | How long stopping an instance may take. A busy VM's instance waits for its job, which the supervisor caps at an hour. |
| `runner_supervisor_source_dir` | `<lab>/automation` | The Go module built on the control node. |
| `runner_supervisor_incus` | `/usr/bin/incus` | The incus CLI on the host. |

## Dependencies

None in `meta/main.yml`. It includes the `incus` role's `tasks/repo.yml`
for the Zabbly repository, so the CLI is the cluster's own line rather
than Debian's older `incus-client`.

## What it does

1. Asserts the token, the VM list, the server certificate, the org, the
   Incus endpoint, the project and the certificate destination are set.
2. Installs `python3-debian`, adds the Zabbly repository and pin, and
   installs `incus-client` and `python3-cryptography`.
3. Builds `./cmd/runner-supervisor` on the control node (`CGO_ENABLED=0`,
   the target's architecture, `-trimpath`, `GOTOOLCHAIN=local`) in a private
   temporary directory, installs it as `/usr/local/bin/runner-supervisor`
   and removes the directory.
4. Generates an ECC client key on the host, under
   `/var/lib/runner-supervisor/incus` (root, `0600`); it never leaves. A
   self-signed client certificate is fetched to `runner_supervisor_cert_dest`.
5. Pins the server certificate and writes the incus CLI's `config.yml` with
   the remote.
6. Writes the token to `/etc/runner-supervisor/token` (root, `0600`).
7. Installs `runner-supervisor@.service` and enables and starts an instance
   per VM. A change to the binary, the token or the unit restarts them.

## Example

```yaml
- name: Runner supervisor
  hosts: runner_supervisor
  become: true
  roles:
    - role: runner_supervisor
      vars:
        runner_supervisor_vms: [docker-runner-1, docker-runner-2]
        runner_supervisor_org: example
        runner_supervisor_remote_address: 192.0.2.10
        runner_supervisor_server_cert: "{{ lookup('file', 'files/incus-server.crt') }}"
        runner_supervisor_project: docker-runners
        runner_supervisor_token: "{{ github_runner_pat }}"
        runner_supervisor_metrics_dir: /var/lib/prometheus/node-exporter
        runner_supervisor_cert_dest: "{{ playbook_dir }}/../../terraform/incus/files/runner-supervisor.crt"
```

## Notes

The unit runs as root with an empty capability set, `ProtectSystem=strict`
and write access only to its state directory and the metrics directory, as
`technitium_dns`'s metrics writer does. Root is for node_exporter's
root-owned textfile directory; without capabilities it can't override file
permissions anywhere else.

The client certificate is useless until the cluster trusts it. A first run
fetches it for the deploy to commit and trust, restricted to the project.
Until then every reset fails, so each instance gives up after its backoff
(about 13 minutes) and exits 3. Run the role again once the trust is
applied: it starts the stopped instances.

Exit code 3 means a VM's reset kept failing and it's left stopped.
`RestartPreventExitStatus=3` keeps systemd from looping on it; `systemctl
restart runner-supervisor@<vm>` retries once the cause is fixed.

Stopping or restarting an instance drains it: an idle VM is deregistered
and stopped, a busy one is stopped once its job ends. A converge that
restarts the instances can therefore wait for a running job.

The incus CLI rewrites its `config.yml` as it's used. The template copies
the CLI's own layout so those rewrites match it. Whether they add keys in
use against a real cluster (such as `last_working_address`) isn't checked
yet, so a converge after first use may report that file changed.

Removing a VM from `runner_supervisor_vms` doesn't stop its instance; run
`systemctl disable --now runner-supervisor@<vm>`.

## Testing

`tests/supervisor.yml` converges the role in a privileged Debian 13 systemd
container for two VMs, with GitHub pointed at a closed local port. It checks
the CLI's version and origin, the binary, the key's mode, the certificate's
subject and that the fetched copy matches, the remote and the pinned server
certificate, the token file, that the build leaves nothing at a fixed
temp path on the control node, and that both instances run, have logged their
retries and never log the token. A second run must change nothing, and
missing input must fail the run. Its header says how to run it.
