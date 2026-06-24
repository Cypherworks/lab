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

## Notes
- The Mongo init script creates the DB user only on MongoDB's first start (empty
  data dir); it's a no-op thereafter.
- Published ports: 8443 (UI), 8080 (inform), 3478/udp (STUN), 10001/udp
  (discovery). The network firewall controls who may reach them.
- Adoption is L3-friendly — point devices at the controller with `set-inform`;
  it need not share their subnet.
