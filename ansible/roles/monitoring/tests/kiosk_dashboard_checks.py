#!/usr/bin/env python3
"""Check a rendered kiosk dashboard's layout and queries.

Run by kiosk_dashboard.yml. Exits non-zero and lists every failure.

Usage: kiosk_dashboard_checks.py <lab-kiosk.json>
"""
import json
import sys

GRID_COLUMNS = 24
# Grafana rows that fit 1080p in kiosk mode.
GRID_ROWS = 28
# A CCTV panel embeds go2rtc's player page.
CCTV_MARKER = "/stream.html?src="
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
# Boxes side by side, named by host alone, so the certs fit one tile row.
CERT_NAME_REGEX = "(?:https://)?([^.]+).*"
CERT_ORIENTATION = "vertical"
CERT_HEIGHT = 3
# Tall enough for several firing alerts at once.
FIRING_MIN_HEIGHT = 6
# Loss has to cover a recent window, and the 6-hourly speedtest can't.
SPEEDTEST_LOSS = "speedtest_tracker_packet_loss_percent"
HOURLY_LOSS = 'probe_success{job="icmp"}[1h]'
SPEEDTEST_PING = "speedtest_tracker_ping_ms"
LIVE_PING = 'probe_icmp_duration_seconds{job="icmp",phase="rtt"}'
# Always firing by design, so counting it would keep the screen red.
FIRING = 'ALERTS{alertstate="firing",alertname!="Watchdog"}'
# Only node-tc1 and node-tc2 report it, so it can't stand for the lab's drives.
PARTIAL_WEAR = "nvme_percentage_used_ratio"
# By name, so an instance mid-migration between nodes counts once.
RUNNING_INSTANCES = "count(count by (name) (incus_procs_total))"
# Each up/down tile and the query that decides it.
SERVICE_TILES = {
    "Auth": 'probe_success{job="service-health",instance="auth"}',
    "Ingress": 'clamp_max(sum(probe_success{job="blackbox-tls"}), 1)',
    "Headscale": 'probe_success{job="service-health",instance="headscale"}',
    "OpenBao": 'probe_success{job="service-health",instance="openbao"}',
    "Vaultwarden": 'probe_success{job="service-health",instance="vaultwarden"}',
    "Home Assistant": 'up{job="hass"}',
}
# Clustered tiles say when a member is down but the service still answers.
CLUSTER_TILES = ("Redis", "etcd", "Patroni", "DNS")
# DEGRADED overflows a 2-wide tile at the kiosk's value size.
CLUSTER_STATES = {"0": "DOWN", "1": "DEGR", "2": "OK"}
# The bottom tiles in order, top row first, 2-wide from x=0.
TILE_ROWS_TOP = 19
TILE_HEIGHT = 3
TILE_ORDER = (
    ("Internet down", "Instances", "Down", "Auth", "Headscale", "NAS", "Patroni"),
    ("Internet up", "Alerts", "Disk faults", "DNS", "Home Assistant", "NAS RAID", "Redis"),
    ("Ping", "Packets lost 1h", "Failed units", "etcd", "Ingress", "OpenBao", "Vaultwarden"),
)
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
    failures = []
    if results != CERT_STATES:
        failures.append(f"cert panel shows {sorted(results)}")
    if CERT_NAME_REGEX not in expiry[0]["targets"][0]["expr"]:
        failures.append("cert panel doesn't name certs by host alone")
    if expiry[0]["options"].get("orientation") != CERT_ORIENTATION:
        failures.append("cert panel doesn't lay its boxes side by side")
    if expiry[0]["gridPos"]["h"] != CERT_HEIGHT:
        failures.append(f"cert panel is {expiry[0]['gridPos']['h']} rows, not {CERT_HEIGHT}")
    return failures


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


def alert_failures(panels):
    exprs = [
        target["expr"] for panel in panels
        for target in panel.get("targets", []) if "ALERTS" in target["expr"]
    ]
    if len(exprs) != 2:
        return [f"expected the tile and the table, found {len(exprs)} alert queries"]
    return [f"counts the heartbeat: {expr}" for expr in exprs if FIRING not in expr]


def wear_failures(panels):
    text = json.dumps([panel.get("targets", []) for panel in panels])
    return ["a panel shows wear for only some drives"] if PARTIAL_WEAR in text else []


def instance_failures(panels):
    text = json.dumps([panel.get("targets", []) for panel in panels])
    return [] if RUNNING_INSTANCES in text else ["no panel counts running instances"]


def stat_tiles(panels):
    return {
        panel["fieldConfig"]["defaults"].get("displayName"): panel
        for panel in panels if panel["type"] == "stat"
    }


def tile_states(tile):
    states = tile["fieldConfig"]["defaults"].get("mappings", [{}])[0].get("options", {})
    return {key: value["text"] for key, value in states.items()}


def service_failures(panels):
    tiles = stat_tiles(panels)
    failures = []
    for name, expr in SERVICE_TILES.items():
        if name not in tiles:
            failures.append(f"no {name} tile")
            continue
        if tiles[name]["targets"][0]["expr"] != expr:
            failures.append(f"{name} reads {tiles[name]['targets'][0]['expr']}")
        if tile_states(tiles[name]) != {"0": "DOWN", "1": "OK"}:
            failures.append(f"{name} doesn't show OK and DOWN")
    return failures


def cluster_failures(panels):
    tiles = stat_tiles(panels)
    failures = []
    for name in CLUSTER_TILES:
        if name not in tiles:
            failures.append(f"no {name} tile")
        elif tile_states(tiles[name]) != CLUSTER_STATES:
            failures.append(f"{name} doesn't show OK, DEGR and DOWN")
    return failures


def tile_order_failures(panels):
    tiles = stat_tiles(panels)
    failures = []
    for row, names in enumerate(TILE_ORDER):
        for column, name in enumerate(names):
            want = {"x": 2 * column, "y": TILE_ROWS_TOP + row * TILE_HEIGHT, "w": 2, "h": TILE_HEIGHT}
            if name not in tiles:
                failures.append(f"no {name} tile")
            elif tiles[name]["gridPos"] != want:
                failures.append(f"{name} sits at {tiles[name]['gridPos']}, not {want}")
    return failures


def firing_table_failures(panels):
    tables = [panel for panel in panels if panel.get("title") == "Firing alerts"]
    if len(tables) != 1:
        return [f"expected one Firing alerts panel, found {len(tables)}"]
    height = tables[0]["gridPos"]["h"]
    return [] if height >= FIRING_MIN_HEIGHT else [f"Firing alerts is only {height} rows tall"]


def main():
    with open(sys.argv[1], encoding="utf-8") as kiosk_file:
        kiosk = json.load(kiosk_file)

    failures = []
    if kiosk.get("uid") != "lab-kiosk":
        failures.append(f"uid is {kiosk.get('uid')!r}")
    failures += layout_failures(kiosk["panels"])
    failures += readability_failures(kiosk["panels"])
    failures += cctv_failures(kiosk["panels"])
    failures += cert_failures(kiosk["panels"])
    failures += loss_failures(kiosk["panels"])
    failures += alert_failures(kiosk["panels"])
    failures += wear_failures(kiosk["panels"])
    failures += instance_failures(kiosk["panels"])
    failures += service_failures(kiosk["panels"])
    failures += cluster_failures(kiosk["panels"])
    failures += tile_order_failures(kiosk["panels"])
    failures += firing_table_failures(kiosk["panels"])

    for failure in failures:
        print(failure)
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
