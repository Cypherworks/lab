# homelab

Reusable infrastructure-as-code for an automated, zero-trust home lab. This is
the public *mechanism* repo: generic, parameterised Terraform modules and Ansible
roles with example values. The live deployment (real topology, data, and
encrypted secrets) lives in a separate private repo that consumes these modules.

## Design in brief

- **Ubuntu 24.04 everywhere** except vendor appliances.
- **Incus** as the virtualization foundation: lightweight system containers for
  always-on services, VMs for heavier and customer workloads.
- **Zero trust**: VLAN segmentation with default-deny inter-VLAN firewalling, SSO
  (Authentik), audited certificate-based access (Teleport), and an identity-gated
  overlay for off-site access.
- **Remote access that survives an uncontrolled upstream NAT**: an overlay control
  plane hosted outside the lab, with connectors that dial outbound, so no inbound
  port-forward is ever required.
- **Encryption at rest by default**: LUKS full-disk encryption with Clevis/Tang
  network-bound unlock, for hardware that lives in a physically untrusted space.
- **Certificates**: automatic Let's Encrypt wildcard via DNS-01.
- **Observability**: VictoriaMetrics + Grafana.
- **Hardening**: CIS Level 2 via the dev-sec collection, verified with OpenSCAP.

## Layout

```
terraform/modules/   reusable Terraform modules
ansible/roles/       reusable Ansible roles (generic, data-free)
scripts/             helper scripts
```

## Mechanism vs data

Every module and role here is written as a generic mechanism: behaviour is
parameterised through variables and defaults, and all site-specific data (hosts,
addresses, users, secrets) is supplied by the consuming deployment, never baked
in. Secrets are handled with SOPS + age and are never committed in plaintext;
the pre-commit guardrails in this repo enforce that.

## Licence

Apache-2.0. See [`LICENSE`](LICENSE).
