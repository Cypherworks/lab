# vector

A per-host log-shipping agent. Installs [Vector](https://vector.dev/) from a pinned,
checksum-verified `.deb` and ships the systemd journal (and any extra file sources) to
VictoriaLogs via its Elasticsearch bulk ingestion endpoint. Run it on every host and
container so system and service logs land in one searchable place.

Part of the `lab` mechanism library: a generic, parameterised role. Supply the VictoriaLogs
endpoint from your inventory; the role holds no site data.

## How it works

The `.deb` ships the `vector` user and a systemd unit. The role adds the `vector` user to
`systemd-journal` so it can read the journal, renders a single config to
`/etc/vector/vector.yaml`, and pins the service to that file with a systemd drop-in (so the
packaged default config can't also load). Logs are grouped into VictoriaLogs streams by
`host` + systemd unit.

## Requirements

- Debian/Ubuntu with systemd; `amd64` or `arm64`.
- Network reachability to the VictoriaLogs endpoint.

## Role variables

| Variable | Default | Purpose |
|---|---|---|
| `vector_version` | `0.56.0` | Pinned Vector version. |
| `vector_arch` | derived | `amd64` on x86_64, else `arm64`. |
| `vector_deb_checksums` | (per-arch) | SHA256 of each `.deb`, verified on download. |
| `vector_victorialogs_endpoint` | `""` | VictoriaLogs `host:port` (e.g. `10.200.30.50:9428`). |
| `vector_stream_fields` | `host,_SYSTEMD_UNIT` | Fields VictoriaLogs groups streams by. |
| `vector_file_sources` | `[]` | Extra file sources: `{ id, path }` (glob). |

## Example

```yaml
- hosts: all
  roles:
    - role: vector
  vars:
    vector_victorialogs_endpoint: "10.0.30.50:9428"
    vector_file_sources:
      - { id: nginx, path: "/var/log/nginx/*.log" }
```
