# esxi

Configures a standalone ESXi 8 host on top of a stock install. Generic mechanism;
all site data comes from the deploy (`build_hosts` group_vars). vCenter itself is
the separate [`vcsa`](../vcsa) role, and identity federation is [`vcenter_oidc`](../vcenter_oidc).

## What it does

1. Sets the host FQDN (`esxi_host_name`) and disables IPv6 (`esxi_disable_ipv6`)
   over SSH. Both need a reboot to apply, so the role reboots when either changed.
2. Turns on the SSH service and sets it to start at boot (`TSM-SSH`, over the API).
3. Local VMFS datastore on a wiped disk. If `esxi_vmfs_device` is unset, discovers
   the lone local non-boot disk and fails closed if there isn't exactly one.
4. NAS storage: mounts a Synology NFS export as an NFS datastore
   (`esxi_nas_server`). Skipped when the server is empty.
5. Wake-on-LAN: arms magic-packet wake on the mgmt uplink so a powered-off host
   can be woken, and adds a re-arm line to `local.sh` so it survives reboots.

## Connection model

This role is not a normal SSH-by-IP role. It mixes two connections:

- **Host API** (`community.vmware`): the service and datastore tasks run against the
  host's vSphere API. They are `delegate_to: localhost` and take `esxi_api_host` /
  `esxi_api_user` / `esxi_api_password`, so they work even before SSH is up.
- **SSH** (raw): hostname, IPv6, disk discovery, and Wake-on-LAN run on the host over
  SSH via `ansible.builtin.raw` (ESXi carries no Python for the normal modules, and
  has no API for these). These run before the API tasks, so **SSH must already be
  enabled** on the host (see Assumptions); the API `TSM-SSH` task only persists it.

Run the play with `gather_facts: false` (ESXi is not a fact-gatherable target) and an
SSH connection as `root` (password from SOPS, `-o StrictHostKeyChecking=accept-new`
on first contact). The API tasks ignore the play connection via `delegate_to`.

## Assumptions

- ESXi 8 is already installed (this does not install ESXi or touch the boot disk).
- **SSH is enabled** (DCUI on a fresh install), reachable as `root`. The role sets
  the hostname over SSH before any API call, so it can't turn SSH on for you first.
  For a truly hands-off rebuild, enable SSH in a kickstart (not yet done here).
- The mgmt vmk / uplink is configured by the installer and reachable.
- Root API credentials work, and the same root password authenticates SSH.
- The disk for the VMFS datastore is wiped (no VMFS to preserve).
- A licensed or in-eval host: all API write operations need write access, which the
  free edition does not grant (eval and licensed hosts do).

## Verify on the box (ESXi-8-specific, not trusted from memory)

- `esxi_host_name` must match how the host identifies itself to the API; an IP that
  does not match the host's configured name can make `esxi_hostname` lookups fail.
- The Wake-on-LAN capability string: this role fails unless `esxcli network nic get`
  reports the uplink supports WoL. Confirm the exact label/value on the box, and note
  that some 8.0U2+ NIC drivers report support but do not actually wake.
- `local.sh` persistence assumes the file ends in `exit 0` (stock ESXi) and that
  `/sbin/auto-backup.sh` persists the edit across reboots.
- Disk discovery parses `esxcli storage core device list` for `Is Local: true` /
  `Is Boot Device: true`; confirm those labels and that the target disk is the only
  local non-boot disk (else set `esxi_vmfs_device`).
- The reboot fires `reboot` with `poll: 0` and waits on port 443; confirm `raw`
  async behaves on the box.

## Key variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `esxi_api_host` | `""` | Address the API modules connect to (the host IP). |
| `esxi_host_name` | `""` | The name ESXi knows itself by (its FQDN). |
| `esxi_api_password` | `""` | Root password (SOPS). |
| `esxi_validate_certs` | `false` | Verify the host cert (self-signed on first run). |
| `esxi_disable_ipv6` | `true` | Disable IPv6 (reboot-applied). |
| `esxi_vmfs_device` | `""` | Canonical disk name; empty auto-discovers the disk. |
| `esxi_vmfs_datastore` | `local-nvme` | Datastore name for the local VMFS. |
| `esxi_nas_server` / `esxi_nas_path` | `""` | NFS server + export; empty skips. |
| `esxi_wol_enabled` | `true` | Arm Wake-on-LAN. |
| `esxi_wol_vmnic` | `""` | Mgmt uplink to arm (e.g. `vmnic0`); empty skips. |
