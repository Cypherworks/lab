# infra_runner

Builds the always-on **converge/infra runner** — the estate's single privileged
executor for `terraform apply` and Ansible converge (lab-deploy D29,
[`execution-model.md`](../../../lab-deploy/docs/execution-model.md)) — on a stock
`images:debian/13/cloud` host. It also serves as the native x86_64 control host
for the VCSA deploy, since the mac OVF Tool can't stream the appliance disks.

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

## Not yet here (follow-up slices)

- Ansible collections (`ansible-galaxy install -r requirements.yml`) and the
  play-specific Python libs (`pyvmomi` for `community.vmware`) and `lego` (the
  `vcsa` cert step) — these depend on the cloned repos, below.
- The two repos cloned side-by-side for `roles_path`, and the run-time GitHub
  identity, which overlap the `claude` role's App-credential and clone machinery;
  whether to share that or duplicate it is a deliberate follow-up decision.
- Self-hosted runner registration and the merge/schedule workflows (phase b/c).

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
  `python3-pyvmomi` vs pip under PEP 668) — resolve when the collections slice lands.

## Example

```yaml
- name: Converge/infra runner
  hosts: infra_runner
  become: true
  roles:
    - infra_runner
```
