# vcsa

Deploys a vCenter Server Appliance (VCSA 8) onto a standalone ESXi host with the
ISO's `vcsa-deploy` CLI, then replaces its self-issued machine SSL certificate with
a CA-signed one. Generic mechanism; all site data comes from the deploy. The ESXi
host is the [`esxi`](../esxi) role; identity federation is [`vcenter_oidc`](../vcenter_oidc).

## Where it runs

On the **control host** (a `localhost` / `connection: local` play), not on the ESXi
host or vCenter. OS-aware: on macOS the role mounts the ISO with `hdiutil` and uses
the `mac/` installer; on Linux it extracts the ISO with `xorriso` to the workdir and
uses the `lin64/` installer (an unprivileged container can't loop-mount, so it can't
mount the ISO — `xorriso` needs `~11 GB` free for the OVA). It needs:

- `vcsa_iso` pointing at the installer ISO on the control host. The role derives
  `vcsa_deploy_bin` (`vcsa-cli-installer/<mac|lin64>/vcsa-deploy`) and
  `vcsa_template_version` (the template's `__version`) from it. On macOS it unmounts
  after; on Linux the extract stays in the workdir. Both vars can be overridden to pin.
- Network reach to the ESXi host (to deploy) and to the vCenter IP (to cert it).
- For the cert step: the `lego` binary (auto-discovered on `PATH`; the ansible run is
  not a login shell, so a Homebrew `lego` may need its dir on `PATH`) and its
  DNS-provider credentials (`vcsa_lego_env`), plus `community.crypto` for the
  live-cert expiry check.

## What it does

1. Renders the `embedded_vCSA_on_ESXi` install spec from the variables and runs
   `vcsa-deploy install` (precheck first). Skipped when vCenter already answers on
   443, since `vcsa-deploy` is not idempotent.
2. Replaces the machine SSL cert (when `vcsa_lego_bin` is set): generates a CSR
   through the API so the private key stays in vCenter, signs it with `lego` over
   DNS-01, and PUTs the cert + CA chain back. Re-issues only when the live cert is
   near expiry or not yet from the expected CA, so convergence runs are a no-op.

## Verify on the box (8-U3-specific, not trusted from memory)

- The ISO layout the discovery assumes: `hdiutil attach -plist` emits a
  `mount-point` for the ISO9660 volume (the mount-point regex reads it), the mac
  installer is at `vcsa-cli-installer/mac/vcsa-deploy`, and the install template is
  `vcsa-cli-installer/templates/install/embedded_vCSA_on_ESXi.json`. A failed deploy
  leaves the ISO mounted until the next successful run's `always` unmount.
- `vcsa_template_version` (read as the template's `__version`) **must** match the
  template vcsa-deploy expects. Diff the rendered `install.json` against the shipped
  template; the precheck is the backstop.
- The `tls-csr` response field the CSR is read from (`vcsa_csr.json.csr`) — confirm
  the exact JSON shape the API returns.
- `lego --csr` writes `<system_name>.crt` and `<system_name>.issuer.crt` under
  `certificates/`; confirm the filenames match `vcsa_system_name`.

## Key variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `vcsa_iso` | `""` | Path to the installer ISO on the control host. |
| `vcsa_deploy_bin` | `""` | Empty auto-discovers `mac/vcsa-deploy` from the ISO. |
| `vcsa_template_version` | `""` | Empty reads `__version` from the ISO template. |
| `vcsa_esxi_host` / `_password` | `""` | Target ESXi host + root password (SOPS). |
| `vcsa_esxi_datastore` | `""` | Datastore the appliance lands on. |
| `vcsa_deployment_option` | `tiny` | Appliance size (smallest single-node). |
| `vcsa_system_name` / `vcsa_ip` | `""` | vCenter FQDN + static IP. |
| `vcsa_gateway` / `vcsa_dns_servers` | `""` / `[]` | Appliance network. |
| `vcsa_os_password` / `vcsa_sso_password` | `""` | Appliance root + SSO admin (SOPS). |
| `vcsa_lego_bin` | `""` | `lego` binary; empty auto-discovers on `PATH`, else skips cert. |
| `vcsa_lego_env` | `{}` | Env for `lego` (DNS-provider creds). |
| `vcsa_le_email` | `""` | ACME account email. |
| `vcsa_cert_renew_before_days` | `30` | Re-issue when fewer days remain. |
| `vcsa_cert_issuer_match` | `Let's Encrypt` | Skip when the live issuer matches. |
