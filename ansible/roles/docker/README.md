# docker

Installs Docker Engine and the Compose v2 plugin from Docker's official APT repository.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian or Ubuntu host. The repo is `download.docker.com/linux/<distro>`, from `ansible_distribution` in lower case.
- Core `ansible.builtin` modules only: `deb822_repository` and `apt`.
- Privilege escalation (`become`) to root.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `docker_packages` | `[docker-ce, docker-ce-cli, containerd.io, docker-compose-plugin]` | Packages installed from Docker's repo. Override to add extras (for example `docker-buildx-plugin`). |

No inventory data or secrets are required.

## Dependencies

None declared in metadata. This role is itself a shared mechanism that service roles (monitoring, speedtest, vaultwarden, authentik_app, github_runner, wol) depend on via `meta`, so the Docker install lives in one place rather than being copied into each.

## What it does

1. Installs `python3-debian` (which `deb822_repository` imports) and `ca-certificates` (to fetch Docker's key), so a minimal image works.
2. Adds Docker's official APT repository for the host's distro as a deb822 source named `docker`, using `ansible_distribution_release` as the suite, `stable` component, and Docker's signing key.
3. Installs `docker_packages` via apt with the cache refreshed.

## Example

```yaml
- hosts: container_hosts
  roles:
    - role: docker
```

## Notes

The repository suite tracks `ansible_distribution_release`, so the host's Debian or Ubuntu release must have a matching Docker package suite upstream. `github_runner/tests/docker_runner.yml` converges this role on both.

This role installs the engine only. It does not manage the Docker daemon service state, users in the `docker` group, or daemon configuration; consuming roles handle their own runtime concerns.
