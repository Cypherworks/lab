# docker

Installs Docker Engine + the Compose v2 plugin from Docker's official APT repository
(Debian). Shared mechanism: the compose-based service roles depend on it via
`meta/main.yml` (`dependencies: [docker]`) rather than each copying the install.

Override `docker_packages` to add packages (e.g. `docker-buildx-plugin`).
