# unifi-switch-ports

Manages an adopted UniFi switch: named port profiles plus per-port overrides, including LACP aggregates, from maps of objects.

Part of the [`lab`](https://github.com/Cypherworks/lab) Terraform module collection. Generic and data-driven: pass a map of objects; the module does `for_each` over it. No site data lives here.

## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.10 |
| unifi (filipowm/unifi) | 1.0.0 |

## Inputs

| Name | Type | Default | Required | Description |
|------|------|---------|:--------:|-------------|
| switch_mac | string | n/a | yes | MAC of the adopted UniFi switch (the device must already be adopted). |
| port_profiles | map(object) | `{}` | no | Reusable named port profiles, keyed by a stable identifier. |
| ports | map(object) | n/a | yes | Per-port assignment keyed by port number (as a string). |
| forget_on_destroy | bool | `true` | no | Forget (not factory-reset) the device when the resource is destroyed. |

`port_profiles` object fields:

| Field | Type | Default | Required | Description |
|-------|------|---------|:--------:|-------------|
| name | string | n/a | yes | Profile name. |
| forward | string | `null` | no | VLAN mode: `all` (trunk all), `native` (access), `customize` (native + tagged), `disabled` (port off). |
| native_network_id | string | `null` | no | Native (untagged) network ID. |
| tagged_vlan_mgmt | string | `null` | no | Tagging policy for `customize` forward mode. |
| excluded_network_ids | set(string) | `null` | no | Networks kept off the trunk in `customize` mode. |
| poe_mode | string | `null` | no | `auto` \| `off` \| `pasv24` \| `passthrough`. |
| op_mode | string | `"switch"` | no | Port operating mode. |
| full_duplex | bool | `null` | no | Force full duplex. |
| speed | number | `null` | no | Link speed. |
| port_security_enabled | bool | `null` | no | Enable MAC port security. |
| port_security_macs | set(string) | `null` | no | Allowed MACs; empty set locks the port down (deny all). |

`ports` object fields:

| Field | Type | Default | Required | Description |
|-------|------|---------|:--------:|-------------|
| profile_key | string | `null` | no | Key into `port_profiles`, resolved to the profile ID. |
| name | string | `null` | no | Port name. |
| poe_mode | string | `null` | no | Per-port PoE override. |
| aggregate_num_ports | number | `null` | no | Bond this port with the following consecutive ports into a LACP aggregate (e.g. `4` = this port + next 3). |

## Outputs

| Name | Description |
|------|-------------|
| port_profile_ids | Map of profile key => UniFi port-profile ID. |
| device_id | ID of the managed switch device. |

## Usage

```hcl
module "switch" {
  source     = "github.com/Cypherworks/lab//terraform/modules/unifi-switch-ports?ref=<commit-sha>"
  switch_mac = "00:11:22:33:44:55"

  port_profiles = {
    access_servers = { name = "access-servers", forward = "native", native_network_id = "000000000000000000000020", poe_mode = "auto" }
    disabled       = { name = "disabled", forward = "disabled", tagged_vlan_mgmt = "block_all", port_security_enabled = true, port_security_macs = [] }
  }

  ports = {
    "5"  = { profile_key = "access_servers", name = "node-1" }
    # NAS LACP across the four consecutive ports 17-20.
    "17" = { profile_key = "access_servers", name = "nas-lag", aggregate_num_ports = 4 }
    # Unused ports shut for zero-trust hygiene.
    "6"  = { profile_key = "disabled" }
  }
}
```

Pin `ref` to a specific commit SHA or tag, never a moving branch.

## Notes

The switch must already be adopted by the controller; this module manages its configuration, it does not adopt it.

An LACP aggregate is one port entry with `aggregate_num_ports` set. The bond covers this port plus the next consecutive ports (`aggregate_num_ports = 4` on port 17 bonds 17-20), so leave the following ports out of `ports`. This supersedes the former `unifi-switch-lag` module.

The provider derives `forward` from the resolved VLAN config (native-only becomes `customize`, the primary LAN becomes `all`), so it never matches the literal sent on create. The profile resource ignores changes to `forward` to keep re-applies clean.

List every port, including a `disabled` profile for unused ones (and SFP ports), for full IaC coverage.

The filipowm/unifi 1.0.0 provider creates resources but cannot update them in place; an edit returns "not found". Apply profile or port changes by rebuild (destroy and recreate the affected resource).
