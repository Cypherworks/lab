#!/usr/bin/env python3
"""Run the rendered Disk faults query against VictoriaMetrics.

Run by disk_faults.yml. Exits non-zero and lists every failure.

Usage: disk_faults_checks.py <lab-kiosk.json> <victoriametrics-url>
"""
import json
import sys
import time
import urllib.parse
import urllib.request

TILE = "Disk faults"
HOUR_SECONDS = 3600
# Each case gets its own hour, so cases can't answer each other's queries.
CASES = (
    ("healthy drives", {}, 0),
    ("NVMe drive raises a critical warning", {"critical_warning": 4}, 1),
    ("NVMe drive reports media errors", {"media_errors": 2}, 1),
    ("NVMe drive does both", {"critical_warning": 4, "media_errors": 2}, 1),
    ("SATA drive fails SMART", {"smart_healthy": 0}, 1),
    ("NAS disk leaves normal", {"disk_status": 2}, 1),
    ("btrfs counts device errors", {"btrfs_errors": 3}, 3),
)


def fleet_samples(timestamp, critical_warning=0, media_errors=0, smart_healthy=1,
                  disk_status=1, btrfs_errors=0):
    samples = (
        f'nvme_critical_warning{{device="nvme0n1",instance="nvme-node"}} {critical_warning}',
        f'nvme_media_errors_total{{device="nvme0n1",instance="nvme-node"}} {media_errors}',
        f'smartmon_device_smart_healthy{{disk="/dev/sda",instance="sata-node",type="sat"}} {smart_healthy}',
        f'synology_disk_status{{diskID="1",instance="nas"}} {disk_status}',
        f'node_btrfs_device_errors_total{{device="/dev/sda",instance="sata-node"}} {btrfs_errors}',
    )
    return "".join(f"{sample} {timestamp * 1000}\n" for sample in samples)


def tile_expr(panels):
    exprs = [
        panel["targets"][0]["expr"] for panel in panels
        if panel.get("fieldConfig", {}).get("defaults", {}).get("displayName") == TILE
    ]
    if len(exprs) != 1:
        raise SystemExit(f"expected one {TILE} tile, found {len(exprs)}")
    return exprs[0]


def tile_value(base_url, expr, timestamp):
    query = urllib.parse.urlencode({"query": expr, "time": timestamp, "nocache": 1})
    with urllib.request.urlopen(f"{base_url}/api/v1/query?{query}") as response:
        result = json.load(response)["data"]["result"]
    return float(result[0]["value"][1]) if result else None


def main():
    with open(sys.argv[1], encoding="utf-8") as kiosk_file:
        expr = tile_expr(json.load(kiosk_file)["panels"])
    base_url = sys.argv[2]

    now = int(time.time())
    timed_cases = [
        (now - (position + 1) * HOUR_SECONDS, name, fault, expected)
        for position, (name, fault, expected) in enumerate(CASES)
    ]
    samples = "".join(fleet_samples(when, **fault) for when, _, fault, _ in timed_cases)
    urllib.request.urlopen(f"{base_url}/api/v1/import/prometheus", samples.encode()).close()
    urllib.request.urlopen(f"{base_url}/internal/force_flush").close()

    failures = []
    for when, name, _, expected in timed_cases:
        value = tile_value(base_url, expr, when)
        if value != expected:
            failures.append(f"{name}: tile reads {value}, not {expected}")

    for failure in failures:
        print(failure)
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
