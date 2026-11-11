# cable_modem_metrics

Writes a DOCSIS cable modem's line state into node_exporter's textfile directory, so the upstream faults behind WAN packet loss show on a graph and can page. It reads the hub's own status replies, which need no login, and keeps no credentials.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. It holds no site data.

## How it works

A timer runs `/usr/local/sbin/cable-modem-metrics`, which reads three replies from `http://<address>/rest/v1/cablemodem/`: `upstream`, `downstream` and `eventlog`. Those paths are the ones a Virgin Media hub's web page calls. They aren't documented, so a firmware update could move them; the collector then fails and the file goes away.

It writes `cable_modem.prom` with these series, each per channel with `channel` and `type` labels:

| Series | What it is |
|---|---|
| `cable_modem_upstream_power_dbmv` | Transmit power. |
| `cable_modem_upstream_modulation_bits` | Bits per symbol: 16-QAM is 4, 64-QAM is 6. A drop means the CMTS stepped the channel down. |
| `cable_modem_upstream_locked` | 1 while locked. |
| `cable_modem_upstream_t3_timeouts_total` | Ranging requests the CMTS never answered: the modem was talking and nobody heard. |
| `cable_modem_upstream_t4_timeouts_total` | Times no station maintenance came from the CMTS. |
| `cable_modem_downstream_power_dbmv`, `cable_modem_downstream_snr_db` | SC-QAM channels only. The hub reports OFDM power and RxMER in units it doesn't name, so they're left out. |
| `cable_modem_downstream_locked` | 1 while locked. |
| `cable_modem_downstream_corrected_total`, `cable_modem_downstream_uncorrected_total` | Codewords repaired and lost. |

From the event log it writes `cable_modem_event_last_timestamp_seconds{kind}`, when the hub last logged each kind of fault: `ranging_timeout` (a T3 timeout), `ranging_gave_up` (retries exhausted on a channel), `power_out_of_range` (the CMTS asked for more transmit power than the modem's window allows) and `reboot`. A kind the log doesn't hold has no sample. The hub keeps only its last few dozen entries, so these timestamps outlive the log. Nothing else from the log is written; its CM and CMTS MACs go nowhere.

`cable_modem_collected_timestamp_seconds` says when the file was written, for a staleness alert.

On any failure (the hub unreachable, an error status, a reply that isn't the JSON it expects, a reply over a megabyte, or a value of the wrong type, such as a number in a string or a lock status that isn't true or false) the collector removes the file rather than leave an old reading, says why in its journal, and the unit fails. The type checks are what stop a reply from adding lines of its own to the file. It runs as root with no capabilities, under a system call filter and a 45-second start timeout, writing only the textfile directory, and its unit allows IPv4 to the hub's address and nothing else.

What the hub serves when it loses sync hasn't been seen. If it answers with empty channel lists, the file still writes and Line faults reads nothing; the WAN loss probes still show it.

## Requirements

- systemd and Python 3.11 or later on the host.
- node_exporter with its textfile collector reading `cable_modem_metrics_dir`.
- A route from the host to the hub's status address.
- Privilege escalation (`become`) to root, set by the play.

## Role variables

| Variable | Default | Purpose |
|---|---|---|
| `cable_modem_metrics_address` | `""` | The hub's status page IPv4 address. Empty removes the collector and its file. A host name is refused, as the unit allows only an address. |
| `cable_modem_metrics_dir` | `/var/lib/prometheus/node-exporter` | node_exporter's textfile directory. |
| `cable_modem_metrics_interval` | `1min` | How often the collector reads the hub. |

## Example

```yaml
- hosts: monitoring
  become: true
  roles:
    - role: cable_modem_metrics
      vars:
        cable_modem_metrics_address: 192.168.100.1
```

## Tests

`tests/metrics.yml` runs the collector under systemd beside Debian's node_exporter in a Debian 13 container, against a second container serving replies captured from a real hub (`tests/hub/`, MACs swapped for documentation ones). It checks that the timer writes the file, the values in it, that node_exporter reads it without a scrape error, that the unit can't reach any other address, that a lost lock or a modulation change shows, that a garbage, missing, oversized or ill-typed reply leaves no file and logs why, that a host name, a trailing newline or an octet over 255 is refused, that a second converge changes nothing, and that turning it off removes it.
