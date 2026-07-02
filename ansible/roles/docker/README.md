# docker

Installs Docker Engine and the Compose v2 plugin from Docker's official APT repository.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian-family host (the repo is configured against `download.docker.com/linux/debian`).
- Core `ansible.builtin` modules only: `deb822_repository` and `apt`.
- Privilege escalation (`become`) to root.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `docker_packages` | `[docker-ce, docker-ce-cli, containerd.io, docker-compose-plugin]` | Packages installed from Docker's repo. Override to add extras (for example `docker-buildx-plugin`). |

No inventory data or secrets are required.

## Dependencies

None declared in metadata. This role is itself a shared mechanism that service roles (monitoring, speedtest, vaultwarden, authentik_app) depend on, so the Docker install lives in one place rather than being copied into each.

## What it does

1. Adds Docker's official APT repository as a deb822 source named `docker`, using `ansible_distribution_release` as the suite, `stable` component, and Docker's signing key.
2. Installs `docker_packages` via apt with the cache refreshed.

## Example

```yaml
- hosts: container_hosts
  roles:
    - role: docker
```

## Notes

The repository suite tracks `ansible_distribution_release`, so the host's Debian release must have a matching Docker package suite upstream.

This role installs the engine only. It does not manage the Docker daemon service state, users in the `docker` group, or daemon configuration; consuming roles handle their own runtime concerns.
