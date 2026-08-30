# infra_runner

Builds the estate's **infra runner** (lab-deploy D29): an x86_64 Linux control
host an operator connects to and runs the estate's Terraform and Ansible plays
from by hand, on a stock `images:debian/13/cloud` host. Not a GitHub Actions
self-hosted runner and not a converge-on-merge executor — a system you SSH into
and run plays on, the Linux in-lab counterpart to the admin MacBook. It is also
the native control host for the VCSA deploy, since the mac OVF Tool can't stream
the appliance disks.

Part of the `lab` mechanism library: a generic, parameterised role. Supply site
data (the age key at run time, AWS credentials, which repos to clone) from your
inventory and SOPS, not from the role.

## What it does

Installs the run toolchain, reusing the checksum-verified download pattern the
`claude` workbench proves on the same image:

1. The distro packages (`ansible`, `git`, `age`, python, pipx, …) via apt.
2. Terraform, from the HashiCorp release zip, verified against `SHA256SUMS`.
3. sops, from the getsops release `.deb`, verified against the release checksums.
4. The AWS CLI, isolated on `PATH` via pipx.
5. Enables `sshd` (the cloud image ships only the client).
6. Clones both repos side-by-side for `roles_path` and installs the galaxy
   collections/roles, using the clone identity from `repos.yml` (the shipped
   `cw-claude-token`/`cw-claude-credential` scripts, reused from the `claude`
   role by reference). The age key is not delivered — supplied at run time (D29).

## Not yet here

- The play-specific `pyvmomi` (for `community.vmware`) and `lego` (the `vcsa`
  cert step) — add when the vCenter play is first run from here.

## Role variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `infra_runner_apt_packages` | see defaults | Distro toolchain (proven on the image by `claude`). |
| `infra_runner_terraform_version` | `1.11.4` | Terraform release, level with the workbench. |
| `infra_runner_sops_version` | `3.9.4` | getsops release; confirm current on the build. |
| `infra_runner_pipx_packages` | `[awscli]` | Python CLIs installed isolated on `PATH`. |

## Verify on the build

- The getsops release asset names (`sops-v<ver>.amd64.deb`, the checksums file):
  confirm they match the release before relying on the download.
- `pyvmomi` on Debian 13 for the system Python that Ansible uses (apt
  `python3-pyvmomi` vs pip under PEP 668) — resolve when the vCenter play runs here.

## Example

```yaml
- name: Infra runner
  hosts: infra_runner
  become: true
  roles:
    - infra_runner
```
