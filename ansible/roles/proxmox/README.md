# proxmox

Configures a standalone Proxmox VE host (PVE 9.x / Debian 13) on top of a stock
install. Generic mechanism; all site data comes from the deploy (`build_hosts`
group_vars). Host login (SSSD) and metrics (`node_exporter`) are separate roles.

## What it does

1. Installs root ops SSH keys (`proxmox_ops_ssh_authorized_keys`).
2. Swaps the enterprise apt repo (401s without a subscription) for the
   no-subscription one (`proxmox_manage_repos`).
3. NVMe fast storage: a VG + thin pool filling `proxmox_nvme_device`, registered
   as PVE `lvmthin` storage. Skipped when the device is empty.
4. NAS storage: mounts a Synology NFS export as PVE storage. Skipped when the
   server is empty.
5. Authentik OIDC: an `openid` realm pointed at the Authentik proxmox app, with
   the groups claim synced onto a pre-created admin group that gets an
   Administrator ACL. Skipped when the issuer URL is empty.

## Assumptions

- PVE is already installed (this does not install PVE or touch the boot disk).
- The management interface / VLAN-aware bridge is configured by the installer.
- Reachable over SSH as `root` for first contact.

## Verify on the box (PVE-9-specific, not trusted from memory)

- apt suite/keyring path (`proxmox_repo_suite`, `proxmox_repo_keyring`).
- `pveum realm add` flag names — especially `--username-claim` (Authentik's
  profile scope emits `preferred_username`) and the groups-sync behaviour.

## Key variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `proxmox_nvme_device` | `""` | NVMe block device for the thin pool; empty skips. |
| `proxmox_nvme_storage_id` | `local-nvme` | PVE storage id for the thin pool. |
| `proxmox_nas_server` / `proxmox_nas_export` | `""` | NFS server + export; empty skips. |
| `proxmox_oidc_issuer_url` | `""` | Authentik app issuer; empty skips OIDC. |
| `proxmox_oidc_client_id` / `_secret` | `""` | OIDC client creds (SOPS). |
| `proxmox_admin_group` | `proxmox-admins` | PVE group the groups claim maps to. |
| `proxmox_ops_ssh_authorized_keys` | `[]` | Root authorized keys. |
