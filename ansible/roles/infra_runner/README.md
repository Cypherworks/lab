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
3. sops, the raw getsops release binary, verified against the release checksums.
4. The AWS CLI, isolated on `PATH` via pipx.
5. mitogen (the fast Ansible strategy) and `pyvmomi` (for `community.vmware`)
   into the system Python with `--break-system-packages` (PEP 668), so they
   import under the interpreter Ansible uses.
6. lego (the `vcsa` cert step), the release tarball binary, on `PATH`.
7. Enables `sshd` (the cloud image ships only the client).
8. Clones both repos side-by-side for `roles_path` and installs the galaxy
   collections/roles, using the clone identity from `repos.yml` (the shipped
   `cw-claude-token`/`cw-claude-credential` scripts, reused from the `claude`
   role by reference). The age key is not delivered — supplied at run time (D29).

## Role variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `infra_runner_apt_packages` | see defaults | Distro toolchain (proven on the image by `claude`). |
| `infra_runner_terraform_version` | `1.11.4` | Terraform release, level with the workbench. |
| `infra_runner_sops_version` | `3.9.4` | getsops release binary; confirm current. |
| `infra_runner_pipx_packages` | `[awscli]` | Python CLIs installed isolated on `PATH`. |
| `infra_runner_pip_packages` | mitogen, pyvmomi | System-python libs Ansible imports. |
| `infra_runner_lego_version` | `5.4.0` | lego release for the `vcsa` cert step. |

## Example

```yaml
- name: Infra runner
  hosts: infra_runner
  become: true
  roles:
    - infra_runner
```
