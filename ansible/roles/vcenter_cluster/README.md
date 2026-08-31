# vcenter_cluster

Creates a Datacenter and a Cluster in vCenter and adds the standalone ESXi host
to that cluster, so node-ryzen is managed by vCenter under a DC/Cluster rather
than sitting outside the inventory. The cluster keeps HA/DRS/vSAN **off** — it
exists only so Packer/TF test code runs against the same DC/Cluster paths as the
client environment (lab-deploy D28). The appliance itself is the [`vcsa`](../vcsa)
role; the standalone host config is [`esxi`](../esxi).

Part of the `lab` mechanism library: a generic, parameterised role. Supply site
data (the DC and cluster names, vCenter and ESXi credentials) from your inventory
and SOPS, not from the role.

## Requirements

- Runs on the control host (a `localhost` / `connection: local` play), after
  `vcsa` has stood vCenter up. `pyvmomi` must be importable by Ansible's Python.
- The vCenter SSO admin credentials, and the ESXi host's root credentials (to add
  the host — its SSL thumbprint is fetched automatically).

## What it does

1. Creates `vcenter_cluster_datacenter` (idempotent).
2. Creates `vcenter_cluster_name` in it — a bare cluster, services left off.
3. Adds `vcenter_cluster_esxi_hostname` into the cluster (`fetch_ssl_thumbprint`).

## Role variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `vcenter_cluster_hostname` | `""` | vCenter host/IP for the API. |
| `vcenter_cluster_username` | `administrator@vsphere.local` | SSO admin. |
| `vcenter_cluster_password` | `""` | SSO admin password (SOPS). |
| `vcenter_cluster_datacenter` | `""` | Datacenter to create. |
| `vcenter_cluster_name` | `""` | Cluster to create (services stay off). |
| `vcenter_cluster_esxi_hostname` | `""` | The ESXi host to add. |
| `vcenter_cluster_esxi_password` | `""` | ESXi root password (SOPS). |

## Example

```yaml
- name: vCenter DC + cluster + host
  hosts: build_hosts
  connection: local
  gather_facts: false
  roles:
    - vcenter_cluster
```
