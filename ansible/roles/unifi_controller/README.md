# unifi_controller

Runs a self-hosted UniFi Network controller plus its MongoDB via docker compose, pinned to the last release that manages the Gen1 USG Pro-4.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- `community.docker` collection.
- Docker Engine and Compose v2 already present on the target host (this role has no `docker` meta dependency).
- `unifi_mongo_password` supplied from SOPS; the play asserts it is set.
- Root via `become`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `unifi_app_image` | `lscr.io/linuxserver/unifi-network-application:8.6.9` | Controller application image. Pinned to 8.6.9, the last release that manages the Gen1 USG Pro-4 (9.x dropped USG support). |
| `unifi_mongo_image` | `mongo:7.0` | MongoDB image. Use an older tag (e.g. `4.4`) on non-AVX hardware. |
| `unifi_data_dir` | `/opt/unifi` | Host directory holding the compose file, Mongo data, and controller config. Override per host (e.g. a NAS share `/volume1/docker/unifi`). |
| `unifi_puid` | `1000` | UID the LinuxServer image drops to. |
| `unifi_pgid` | `1000` | GID the LinuxServer image drops to. |
| `unifi_mongo_user` | `unifi` | MongoDB user the controller connects as. |
| `unifi_mongo_dbname` | `unifi` | MongoDB database the controller uses. |
| `unifi_mongo_shell` | `mongosh` | Mongo client used to create the DB user. `mongo:6.0+` ships `mongosh`; older tags ship the legacy `mongo` shell. |
| `unifi_ports` | `["8443:8443", "8080:8080", "3478:3478/udp", "10001:10001/udp"]` | Host port publishings for the UI/device comms (8443), device inform (8080), STUN (3478), and device discovery (10001). |

`unifi_mongo_password` has no default and MUST be supplied from SOPS. `unifi_docker_cli` is an optional override (see Notes).

## Dependencies

None declared. Docker Engine and Compose v2 must already be installed on the host.

## What it does

Asserts `unifi_mongo_password` is set, then creates `unifi_data_dir` and its `mongo/` and `config/` bind-mount source directories. It renders the compose file and brings the two-container stack (application plus MongoDB) up with `docker_compose_v2`. Finally it creates the controller's MongoDB user by piping `mongo-init.js` into the DB container over `docker exec` (retried until Mongo is up); the script is idempotent via a `getUser` guard, and the task carries `no_log` because the script contains the Mongo password.

## Example

```yaml
- hosts: unifi_host
  roles:
    - role: unifi_controller
      vars:
        unifi_mongo_password: "{{ vault_unifi_mongo_password }}"
```

On a Synology NAS:

```yaml
- hosts: synology
  roles:
    - role: unifi_controller
      vars:
        unifi_data_dir: /volume1/docker/unifi
        unifi_docker_cli: /usr/local/bin/docker
        unifi_mongo_password: "{{ vault_unifi_mongo_password }}"
```

## Notes

The role is Synology-aware. Synology's Docker won't auto-create bind-mount source directories, so the role creates `mongo/` and `config/` explicitly. The MongoDB user is created via `docker exec` and the Mongo shell rather than the `/docker-entrypoint-initdb.d` hook, because `@eaDir` metadata in the data directory stops Mongo treating it as fresh, so that hook never fires on Synology. Set `unifi_docker_cli` when the Docker binary is not on the default `PATH` (as on DSM).

The lab firewall governs which sources may actually reach the published ports.
