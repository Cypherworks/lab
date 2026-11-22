# github_jit_runner

Installs a self-hosted GitHub Actions runner natively from the pinned
`actions/runner` tarball, ready to run one job from a just-in-time (JIT)
config. Nothing registers at deploy time and the role holds no token: a
supervisor outside the host mints the JIT config and hands it in with
`start-jit-runner`. Built for VMs that are reset to a clean snapshot after
every job; the container-based `github_runner` role stays for hosts that
register themselves with a PAT.

## Requirements

Ubuntu 24.04 with systemd, as root (or `become`). The package names in
`github_jit_runner_packages` are Ubuntu 24.04's; other releases need their
own libicu and libssl names. Internet access to `github.com` for the
tarball. The tarball is linux-x64 only.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `github_jit_runner_version` | `2.338.2` | Runner release to install. |
| `github_jit_runner_sha256` | the v2.338.2 linux-x64 checksum | SHA-256 of the tarball, from the release notes. Must be 64 lowercase hex digits. |
| `github_jit_runner_release_base` | `https://github.com/actions/runner/releases/download` | Release download base. |
| `github_jit_runner_archive` | `actions-runner-linux-x64-<version>.tar.gz` | Tarball file name. |
| `github_jit_runner_url` | built from the three above | Tarball URL. |
| `github_jit_runner_dir` | `/opt/actions-runner` | Each version unpacks to `<dir>/<version>`; every other directory under it is removed. |
| `github_jit_runner_user` | `runner` | The user jobs run as. Owns the unpacked runner. |
| `github_jit_runner_groups` | `[]` | Extra groups for that user, such as `[docker]` where jobs drive the host's daemon. The groups must exist. |
| `github_jit_runner_sudo` | `false` | Passwordless sudo for that user (see Notes). |
| `github_jit_runner_packages` | `libkrb5-3`, `zlib1g`, `liblttng-ust1t64`, `libssl3t64`, `libicu74` | The runner's runtime libraries, as its `bin/installdependencies.sh` picks them on Ubuntu 24.04. |

## Dependencies

None. Install Docker (the `docker` role) before this one when jobs need it,
so the `docker` group exists for `github_jit_runner_groups`.

## What it does

1. Asserts the checksum is well formed.
2. Installs the runtime libraries, and `sudo` when sudo is on.
3. Creates the job user with its extra groups, and sets or removes
   `/etc/sudoers.d/github-jit-runner` (checked with `visudo`).
4. When `<dir>/<version>/run.sh` is missing, downloads the tarball with its
   checksum checked, unpacks it as the job user's, and removes the download.
   Other versions under `<dir>` are removed.
5. Installs `actions-runner.service`. It has no `[Install]` section, so it
   is never enabled and a boot never starts it.
6. Installs `/usr/local/sbin/start-jit-runner` (root, `0700`).

`start-jit-runner` reads a JIT config from stdin, writes it to
`/run/actions-runner/jitconfig` (tmpfs, owned by the job user, `0400`) and
starts the unit, which runs `run.sh --jitconfig <config>` as the job user.
It refuses empty stdin, leaving nothing behind, and refuses a second config
in the same boot.

## Example

```yaml
- name: Docker runner VMs
  hosts: docker_runner
  become: true
  roles:
    - role: docker
    - role: github_jit_runner
      vars:
        github_jit_runner_groups: [docker]
        github_jit_runner_sudo: true
```

The supervisor then hands in a config it minted:

```bash
incus exec <remote>:<vm> -- /usr/local/sbin/start-jit-runner < jitconfig
```

## Notes

The JIT config is the runner's only credential and is good for one job. The
job runs as the job user, which can read the config file and see it in
`run.sh`'s argv, so it's no more protected than the job's own token. That's
why the host must be reset before it registers again, and why the script
refuses a second hand-off.

Passwordless sudo matches GitHub-hosted runners, so workflows that `apt-get`
work unchanged. Where the job user is in the `docker` group it's
root-equivalent anyway. Turn sudo on only on a host that's reset after every
job.

`--jitconfig` isn't in GitHub's runner docs. It's a `run` option in the
runner source (`src/Runner.Listener/CommandSettings.cs`), and a JIT runner
started this way came online in 5.2 seconds on the lab's Docker runner VM
(2026-11-22).

GitHub stops assigning jobs to a runner more than 63 days behind the latest
release, so bump `github_jit_runner_version` and its checksum together.

## Testing

`tests/jit_runner.yml` converges the role in a privileged Ubuntu 24.04
systemd container with sudo on. It checks the packages, the user and its
groups, sudo, the unpacked runner's owner, that the unit isn't enabled,
that a stale version is removed and a further run changes nothing. It then
swaps `run.sh` for a stub and drives `start-jit-runner` as a supervisor
would: empty stdin, one config, then a second. A run with sudo off must take
sudo away, and a malformed checksum must fail the run. Its header says how
to run it.
