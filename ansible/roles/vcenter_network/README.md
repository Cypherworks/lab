# vcenter_network

Manages VM port groups on an existing vSphere Distributed Switch (vDS) in vCenter.
Each port group gets its VLAN, its port binding, and a closed security policy
(promiscuous, forged transmits and MAC changes all rejected, not inherited from
the switch). Existing port groups with the same name are adopted and brought to
that state.

The switch itself is out of scope: uplinks, host membership and vmkernel
migration must already be in place. The appliance is the [`vcsa`](../vcsa) role,
and the DC/cluster/host is [`vcenter_cluster`](../vcenter_cluster).

Part of the `lab` mechanism library: a generic, parameterised role. Supply site
data (the switch name, port groups, vCenter credentials) from your inventory and
SOPS, not from the role.

## Requirements

- Runs on the control host (a `localhost` / `connection: local` play), after
  vCenter is up and the vDS exists. `pyvmomi` must be importable by Ansible's
  Python.
- The vCenter SSO admin credentials.

## What it does

1. For each entry in `vcenter_network_portgroups`, ensures a port group with that
   name exists on `vcenter_network_dvswitch` with the given VLAN and binding, and
   the security policy below (idempotent).

It never removes port groups that aren't listed.

## Role variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `vcenter_network_hostname` | `""` | vCenter host/IP for the API. |
| `vcenter_network_username` | `administrator@vsphere.local` | SSO admin. |
| `vcenter_network_password` | `""` | SSO admin password (SOPS). |
| `vcenter_network_validate_certs` | `false` | Verify the vCenter API certificate. |
| `vcenter_network_dvswitch` | `""` | The existing vDS the port groups live on. |
| `vcenter_network_portgroups` | `[]` | `[{name, vlan_id, port_binding?}]`. |
| `vcenter_network_port_binding` | `static` | Binding when an entry sets none. `ephemeral` lets VMs attach from the host client while vCenter is down. |
| `vcenter_network_promiscuous` | `false` | Allow promiscuous mode. |
| `vcenter_network_forged_transmits` | `false` | Allow forged transmits. |
| `vcenter_network_mac_changes` | `false` | Allow MAC address changes. |

## Example

```yaml
- name: vCenter port groups
  hosts: build_hosts
  connection: local
  gather_facts: false
  vars:
    vcenter_network_dvswitch: vDS01
    vcenter_network_portgroups:
      - { name: vDS-Customer, vlan_id: 45 }
  roles:
    - vcenter_network
```
