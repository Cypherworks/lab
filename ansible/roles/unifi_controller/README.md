# unifi_controller

Self-hosted UniFi Network controller (app + MongoDB) via `docker compose`, pinned
to `8.6.9` — the last release that manages the Gen1 USG Pro-4 (9.x dropped USG
support).

## Requirements (on the target Docker host)
- Docker with Compose v2 (`docker compose`).
- A Python interpreter Ansible can use (set `ansible_python_interpreter` if it's
  not at the default path — e.g. on Synology after installing the Python3 package).
- The connecting user able to run Docker (root, or sudo via `become`).

## Required vars
- `unifi_mongo_password` — MongoDB user password. Supply from SOPS; never commit.
- `unifi_data_dir` — host path for the compose file + data (e.g. `/volume1/docker/unifi`).

## Optional vars
- `unifi_docker_cli` — explicit path to the `docker` binary. Set this when Docker
  isn't on the module's exec PATH (e.g. Synology, where it's `/usr/local/bin/docker`).
- `unifi_mongo_shell` — Mongo client for user creation: `mongosh` (mongo:6.0+,
  default) or `mongo` (4.4 and older, e.g. non-AVX hardware).

## Notes
- The Mongo DB user is created by the role (idempotent `docker exec` after the
  stack is up), not via Mongo's entrypoint init dir — on Synology, `@eaDir`
  metadata stops Mongo treating the data dir as fresh, so that hook never fires.
- Published ports: 8443 (UI), 8080 (inform), 3478/udp (STUN), 10001/udp
  (discovery). The network firewall controls who may reach them.
- Adoption is L3-friendly — point devices at the controller with `set-inform`;
  it need not share their subnet.
