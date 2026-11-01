# journal_pull

Copies a host's systemd journal to the control host as gzipped JSON, for a host that can't ship its own logs: one with no egress, where a log path out would be the first hole in it. The control host initiates, so the host needs no new outbound rule and no agent.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. It holds no site data.

## How it works

The journal is exported to a private scratch directory on the host, fetched to `journal_pull_dir/<host>-journal-<UTC time>.json.gz` with mode `0600`, and the scratch directory is removed whether the pull worked or not. A failed export fails the run, so a truncated file can't pass as a pull. Each pull is the whole journal, so the newest holds everything the host still has; the older ones keep what it has since rotated away, or lost in a rebuild.

Read one with `gzip -dc <file> | jq -r .MESSAGE`.

## Requirements

- systemd, gzip and bash on the host.
- Privilege escalation (`become`) to root on the host, as the journal isn't world-readable.

## Role variables

| Variable | Default | Purpose |
|---|---|---|
| `journal_pull_dir` | `~/.local/state/journal_pull` | Where pulls land on the control host (mode `0700`). |
| `journal_pull_keep` | `10` | How many pulls to keep per host. |

## Example

```yaml
- hosts: airgapped
  tasks:
    - name: Pull the journal
      ansible.builtin.include_role:
        name: journal_pull
```

## Tests

`tests/pull.yml` pulls a real journal from systemd in a Docker container and checks the content, the mode, the rotation, and that a failing `journalctl` fails the run and leaves nothing behind.
