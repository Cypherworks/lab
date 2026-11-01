#!/usr/bin/env python3
"""Check a rendered kiosk dashboard against a rendered Lab Overview.

Run by kiosk_dashboard.yml. Exits non-zero and lists every failure.

Usage: kiosk_dashboard_checks.py <lab-kiosk.json> <lab-overview.json>
"""
import json
import sys

GRID_COLUMNS = 24
# Grafana rows that fit 1080p in kiosk mode.
GRID_ROWS = 28
# Panels holding these must match Lab Overview: CCTV, power and NAS.
CCTV_MARKER = "/stream.html?src="
SHARED_MARKERS = (CCTV_MARKER, "hass_sensor_", "synology_")
# Grid cell and gutter in pixels, measured on 1920x1080 at Grafana 11.2.
COLUMN_PX = 79
ROW_PX = 38
GUTTER_PX = 8
# Padding Grafana keeps around a text panel's content, measured likewise.
PANEL_PADDING_WIDTH_PX = 16
PANEL_PADDING_HEIGHT_PX = 18
# Each camera's real picture shape, by go2rtc stream name.
CCTV_ASPECTS = {"front": 1536 / 432, "garage_carport": 896 / 512}
# A box this close, as a fraction, leaves no visible bars.
CCTV_ASPECT_TOLERANCE = 0.03
CERT_STATES = {"OK", "Expiring", "Expired"}
# Loss has to cover a recent window, and the 6-hourly speedtest can't.
SPEEDTEST_LOSS = "speedtest_tracker_packet_loss_percent"
HOURLY_LOSS = 'probe_success{job="icmp"}[1h]'
SPEEDTEST_PING = "speedtest_tracker_ping_ms"
LIVE_PING = 'probe_icmp_duration_seconds{job="icmp",phase="rtt"}'
# Below this a tile's name and value stop being readable from a distance.
MIN_WIDTH = 2
MIN_HEIGHT = 2


def cells(panel):
    pos = panel["gridPos"]
    return {
        (x, y)
        for x in range(pos["x"], pos["x"] + pos["w"])
        for y in range(pos["y"], pos["y"] + pos["h"])
    }


def layout_failures(panels):
    failures = []
    ids = [panel["id"] for panel in panels]
    if len(ids) != len(set(ids)):
        failures.append(f"duplicate panel ids: {sorted(ids)}")

    owner = {}
    for panel in panels:
        for x, y in cells(panel):
            if x >= GRID_COLUMNS or y >= GRID_ROWS:
                failures.append(f"panel {panel['id']} leaves the screen at {x},{y}")
            if (x, y) in owner:
                failures.append(f"panels {owner[x, y]} and {panel['id']} overlap at {x},{y}")
            owner[x, y] = panel["id"]

    empty = GRID_COLUMNS * GRID_ROWS - len(owner)
    if empty:
        failures.append(f"{empty} grid cells are unused")
    return failures


def is_cctv(panel):
    return CCTV_MARKER in panel["options"].get("content", "")


def readability_failures(panels):
    failures = []
    for panel in panels:
        pos = panel["gridPos"]
        if pos["w"] < MIN_WIDTH or pos["h"] < MIN_HEIGHT:
            failures.append(f"panel {panel['id']} is squashed to {pos['w']}x{pos['h']}")
    return failures


def cctv_failures(panels):
    failures = []
    for panel in filter(is_cctv, panels):
        pos = panel["gridPos"]
        width = pos["w"] * COLUMN_PX - GUTTER_PX - PANEL_PADDING_WIDTH_PX
        height = pos["h"] * ROW_PX - GUTTER_PX - PANEL_PADDING_HEIGHT_PX
        stream = panel["options"]["content"].split("src=")[2].split("&")[0]
        if abs(width / height / CCTV_ASPECTS[stream] - 1) > CCTV_ASPECT_TOLERANCE:
            failures.append(f"panel {panel['id']} is {width}x{height}, not {stream}'s shape")
        if panel["title"]:
            failures.append(f"panel {panel['id']} loses height to its title")
    return failures


def cert_failures(panels):
    expiry = [
        panel for panel in panels
        if "probe_ssl_earliest_cert_expiry" in json.dumps(panel.get("targets", []))
    ]
    if len(expiry) != 1:
        return [f"expected one cert panel, found {len(expiry)}"]
    mappings = expiry[0]["fieldConfig"]["defaults"].get("mappings", [])
    results = {m["options"]["result"]["text"] for m in mappings if m["type"] == "range"}
    if results != CERT_STATES:
        return [f"cert panel shows {sorted(results)}"]
    return []


def loss_failures(panels):
    text = json.dumps([panel.get("targets", []) for panel in panels])
    failures = []
    if SPEEDTEST_LOSS in text:
        failures.append("loss still reads the last speedtest")
    if HOURLY_LOSS.replace('"', '\\"') not in text:
        failures.append("no panel counts loss over the last hour")
    if SPEEDTEST_PING in text:
        failures.append("ping still reads the last speedtest")
    if LIVE_PING.replace('"', '\\"') not in text:
        failures.append("no panel shows live ping")
    return failures


def is_shared(panel):
    text = json.dumps({k: v for k, v in panel.items() if k in ("targets", "options")})
    return any(marker in text for marker in SHARED_MARKERS)


def without_layout(panel):
    options = {k: v for k, v in panel["options"].items() if k != "text"}
    kept = {k: v for k, v in panel.items() if k not in ("gridPos", "title")}
    return kept | {"options": options}


def shared_panel_failures(kiosk_panels, overview_panels):
    kiosk = {panel["id"]: panel for panel in kiosk_panels}
    failures = []
    for panel in overview_panels:
        if not is_shared(panel):
            continue
        if panel["id"] not in kiosk:
            failures.append(f"Lab Overview panel {panel['id']} is missing")
        elif without_layout(kiosk[panel["id"]]) != without_layout(panel):
            failures.append(f"panel {panel['id']} differs from Lab Overview")
    return failures


def main():
    with open(sys.argv[1], encoding="utf-8") as kiosk_file:
        kiosk = json.load(kiosk_file)
    with open(sys.argv[2], encoding="utf-8") as overview_file:
        overview = json.load(overview_file)

    failures = []
    if kiosk.get("uid") != "lab-kiosk":
        failures.append(f"uid is {kiosk.get('uid')!r}")
    failures += layout_failures(kiosk["panels"])
    failures += readability_failures(kiosk["panels"])
    failures += cctv_failures(kiosk["panels"])
    failures += cert_failures(kiosk["panels"])
    failures += loss_failures(kiosk["panels"])
    failures += shared_panel_failures(kiosk["panels"], overview["panels"])

    for failure in failures:
        print(failure)
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
