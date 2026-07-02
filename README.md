# lab

A library of generic, parameterised infrastructure-as-code for building a secure-by-design network, virtualisation platform, and service estate. It holds mechanism only: Ansible roles and Terraform modules with no site data. Addresses, hostnames, account IDs, and secrets are supplied by a separate private deployment repository that consumes this one.

The code was written for a defence/government-oriented home lab and follows secure-by-design and zero-trust principles throughout, but every role and module is a reusable building block you can adopt in your own environment.

## Design principles

- **Mechanism, not data.** Roles and modules take behaviour from variables and defaults. Site-specific values (IPs, users, secrets) stay in the consuming repository's inventory, `group_vars`/`host_vars`, and SOPS-encrypted files. Nothing in this repository is specific to one deployment.
- **Secure by design.** Default-deny where a choice exists: signups off, admin panels disabled until a token is set, additive SSH changes that never weaken authentication, secrets passed by environment file rather than written to config, and validate-before-reload on every service that supports a config check.
- **Idempotent and safe to re-run.** Destructive or identity-bearing operations are guarded so a re-run can never repeat them: certificate authorities are generated exactly once, database clusters are protected by marker files, and clustered services that rewrite their own configuration are written once and left alone.
- **Continuous re-assertion.** The design assumes these roles and modules are applied repeatedly to converge state, not run once. Steady state is a clean no-op.

## Layout

```
ansible/roles/     26 roles — OS baseline, DNS, ingress, HA data tier, virtualisation, identity, secrets, observability
terraform/modules/ 5 UniFi network modules — networks, firewall, switch ports, port forwards, WLANs
scripts/           host imaging and operational helpers
```

## Ansible roles

Each role has its own README with variables, dependencies, and an example. Roles are grouped by function below.

### OS baseline and access

| Role | Purpose |
|------|---------|
| [`base`](ansible/roles/base) | Hostname, packages, timezone, operator SSH keys, static netplan. Applied to every host. |
| [`docker`](ansible/roles/docker) | Docker Engine + Compose v2 from Docker's APT repository. Shared dependency. |
| [`unattended_upgrades`](ansible/roles/unattended_upgrades) | Automatic security updates with a quiet-window reboot. |
| [`ssh_ca_trust`](ansible/roles/ssh_ca_trust) | Trust an SSH certificate authority for user certificates via an additive sshd drop-in. |
| [`sssd`](ansible/roles/sssd) | SSSD LDAP client against an Authentik LDAP outpost; identity by default, optional PAM login. |
| [`tailscale`](ansible/roles/tailscale) | Join a host to a Headscale/Tailscale overlay. |

### DNS, network, and ingress

| Role | Purpose |
|------|---------|
| [`blocky`](ansible/roles/blocky) | DNS frontend with blocklists and local records, forwarding to a local recursive resolver. |
| [`unbound`](ansible/roles/unbound) | Recursive validating resolver on loopback, behind blocky. |
| [`keepalived`](ansible/roles/keepalived) | VRRP floating IP across DNS nodes with a real DNS-query health check. |
| [`caddy`](ansible/roles/caddy) | Manage the Caddy reverse-proxy configuration and trusted internal CAs. |
| [`sni_router`](ansible/roles/sni_router) | L4 SNI passthrough router (nginx stream) that terminates no TLS. |
| [`headscale`](ansible/roles/headscale) | Headscale control server from a pinned upstream package, with optional OIDC. |

### High-availability data tier

| Role | Purpose |
|------|---------|
| [`etcd`](ansible/roles/etcd) | etcd cluster serving as the Patroni distributed configuration store. |
| [`postgres_patroni`](ansible/roles/postgres_patroni) | PostgreSQL under Patroni with automatic failover via etcd. |
| [`haproxy_patroni`](ansible/roles/haproxy_patroni) | Node-local HAProxy routing to the current Patroni primary and Redis master. |
| [`redis_sentinel`](ansible/roles/redis_sentinel) | Redis with Sentinel for automatic master promotion. |

### Virtualisation, secrets, identity

| Role | Purpose |
|------|---------|
| [`incus`](ansible/roles/incus) | Incus with web UI, per-VLAN bridges, dedicated storage, and optional clustering. |
| [`openbao`](ansible/roles/openbao) | OpenBao secrets manager — PKI, AWS-KMS auto-unseal, OIDC, SSH CA, S3 snapshots. |
| [`authentik`](ansible/roles/authentik) | Authentik identity provider, standalone all-in-one deployment. |
| [`authentik_app`](ansible/roles/authentik_app) | Authentik app tier against an external HA database, with blueprint integrations and an LDAP outpost. |

### Observability and applications

| Role | Purpose |
|------|---------|
| [`monitoring`](ansible/roles/monitoring) | VictoriaMetrics, vmalert, Alertmanager, ntfy, Grafana, and exporters on one host. |
| [`node_exporter`](ansible/roles/node_exporter) | Prometheus node_exporter on bare-metal hosts. |
| [`unifi_controller`](ansible/roles/unifi_controller) | Self-hosted UniFi Network controller and MongoDB. |
| [`vaultwarden`](ansible/roles/vaultwarden) | Vaultwarden password manager. |
| [`speedtest`](ansible/roles/speedtest) | Speedtest Tracker with a Prometheus endpoint. |

### Raspberry Pi hardware

| Role | Purpose |
|------|---------|
| [`rpi_poe_fan`](ansible/roles/rpi_poe_fan) | Quiet PoE HAT fan temperature thresholds. |
| [`rpi_radios`](ansible/roles/rpi_radios) | Disable onboard WiFi and Bluetooth at firmware level. |

## Terraform modules

Data-driven UniFi network modules. Each takes a map of objects and iterates with `for_each`. See [`terraform/modules`](terraform/modules) for the collection overview and the provider decision.

| Module | Purpose |
|--------|---------|
| [`unifi-networks`](terraform/modules/unifi-networks) | VLANs from a data map. |
| [`unifi-firewall`](terraform/modules/unifi-firewall) | USG firewall groups and rules. |
| [`unifi-switch-ports`](terraform/modules/unifi-switch-ports) | Full switch port configuration, including LACP aggregates. |
| [`unifi-port-forwards`](terraform/modules/unifi-port-forwards) | WAN-to-LAN port forwards. |
| [`unifi-wlans`](terraform/modules/unifi-wlans) | WLANs/SSIDs. |

All modules require Terraform `>= 1.10` and provider `filipowm/unifi` `1.0.0`.

## Scripts

| Script | Purpose |
|--------|---------|
| [`flash-pi.sh`](scripts/flash-pi.sh) | Write an Ubuntu arm64 image to SD/USB and inject headless cloud-init for a Raspberry Pi. macOS only. |
| [`flash-x86.sh`](scripts/flash-x86.sh) | Build one generic automated Ubuntu 24.04 autoinstall USB/ISO for all x86 hosts. macOS only. |
| [`provision-poller.sh`](scripts/provision-poller.sh) | Wait for freshly-flashed hosts to answer SSH and run a smoke check. |
| [`bao-ssh-sign.sh`](scripts/bao-ssh-sign.sh) | Sign an SSH public key with the OpenBao SSH CA for short-lived, identity-locked access. |
| [`check-sops-encrypted.sh`](scripts/check-sops-encrypted.sh) | Pre-commit guard that fails if a file that should be SOPS-encrypted is staged in plaintext. |

## Using this in your own environment

These roles and modules are consumed by a separate private repository that holds your site data. There is no `requirements.yml` in this repository; roles are referenced directly.

**Ansible.** Clone this repository alongside your deployment repository and point `roles_path` at it in your `ansible.cfg`:

```ini
[defaults]
roles_path = roles:../lab/ansible/roles
```

Supply every value marked as site data or a secret in the role READMEs from your own inventory and SOPS-encrypted vars. The roles target Ubuntu (22.04/24.04) and Debian 13 hosts and use the `community.general`, `ansible.posix`, `community.docker`, and `community.postgresql` collections.

**Terraform.** Reference modules by Git source, pinned to a specific commit or tag:

```hcl
module "networks" {
  source = "github.com/Cypherworks/lab//terraform/modules/unifi-networks?ref=<commit-sha>"
  # ...
}
```

Pin `ref` to an immutable commit or tag, never a moving branch.

## Prerequisites

- Ansible (2.15+) with the collections listed above
- Terraform `>= 1.10`
- SOPS with an age key for encrypted variables
- For host imaging: macOS with `xorriso` (x86) and admin rights to write removable media

## Security posture

The roles implement segmentation-friendly defaults, identity-gated access, no standing credentials in code, and CIS-aligned hardening applied by the consuming repository. Secrets are expected to be SOPS-encrypted at rest and delivered to hosts by environment file, never committed in plaintext. The full trust model and control set live in the private deployment repository's design documentation.

No warranty. Review every role and module against your own threat model and change control before use.
