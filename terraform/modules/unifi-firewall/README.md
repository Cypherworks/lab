# unifi-firewall

Creates UniFi firewall address/port groups and legacy USG firewall rules from maps of objects.

Part of the [`lab`](https://github.com/Cypherworks/lab) Terraform module collection. Generic and data-driven: pass a map of objects; the module does `for_each` over it. No site data lives here.

## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.10 |
| unifi (filipowm/unifi) | 1.0.0 |

## Inputs

| Name | Type | Default | Required | Description |
|------|------|---------|:--------:|-------------|
| firewall_groups | map(object) | `{}` | no | Address/port groups keyed by a stable identifier. Rules reference these by key; the module resolves keys to IDs internally. |
| rules | map(object) | n/a | yes | Legacy USG firewall rules keyed by a stable identifier. |

`firewall_groups` object fields:

| Field | Type | Default | Required | Description |
|-------|------|---------|:--------:|-------------|
| name | string | n/a | yes | Group name. |
| type | string | n/a | yes | e.g. `address-group`, `port-group`, `ipv6-address-group`. |
| members | list(string) | n/a | yes | Group members (IPs, CIDRs, or ports). |

`rules` object fields:

| Field | Type | Default | Required | Description |
|-------|------|---------|:--------:|-------------|
| name | string | n/a | yes | Rule name. |
| ruleset | string | n/a | yes | USG ruleset, e.g. `LAN_IN`, `LAN_LOCAL`, `GUEST_IN`, `WAN_IN`. |
| rule_index | number | n/a | yes | Ordering index within the ruleset. |
| action | string | n/a | yes | `accept` \| `drop` \| `reject`. |
| enabled | bool | `true` | no | Whether the rule is active. |
| logging | bool | `false` | no | Log matches. |
| protocol | string | `null` | no | Protocol match. |
| src_network_id | string | `null` | no | Source network ID. |
| src_address | string | `null` | no | Source address or CIDR. |
| src_group_keys | list(string) | `null` | no | Keys into `firewall_groups`, resolved to source group IDs. |
| src_port | string | `null` | no | Source port. |
| dst_network_id | string | `null` | no | Destination network ID. |
| dst_address | string | `null` | no | Destination address or CIDR. |
| dst_group_keys | list(string) | `null` | no | Keys into `firewall_groups`, resolved to destination group IDs. |
| dst_port | string | `null` | no | Destination port. |
| state_established | bool | `null` | no | Match established connections. |
| state_related | bool | `null` | no | Match related connections. |
| state_invalid | bool | `null` | no | Match invalid connections. |
| state_new | bool | `null` | no | Match new connections. |

## Outputs

| Name | Description |
|------|-------------|
| firewall_group_ids | Map of group key => UniFi firewall group ID. |
| firewall_rule_ids | Map of rule key => UniFi firewall rule ID. |

## Usage

```hcl
module "firewall" {
  source = "github.com/Cypherworks/lab//terraform/modules/unifi-firewall?ref=<commit-sha>"

  firewall_groups = {
    cameras = {
      name    = "cctv-cameras"
      type    = "address-group"
      members = ["192.0.2.10", "192.0.2.11"]
    }
  }

  rules = {
    allow_cameras_to_nas = {
      name           = "cameras-to-nas-nfs"
      ruleset        = "LAN_IN"
      rule_index     = 2010
      action         = "accept"
      protocol       = "tcp"
      src_group_keys = ["cameras"]
      dst_address    = "10.0.20.30"
      dst_port       = "2049"
    }
    drop_inter_vlan = {
      name        = "drop-inter-vlan"
      ruleset     = "LAN_IN"
      rule_index  = 2999
      action      = "drop"
      src_address = "10.0.0.0/8"
      dst_address = "10.0.0.0/8"
    }
  }
}
```

Pin `ref` to a specific commit SHA or tag, never a moving branch.

## Notes

The USG Pro-4 on a self-hosted controller uses legacy rulesets (`LAN_IN`, `LAN_LOCAL`, `GUEST_IN`, `WAN_IN`, etc.) ordered by `rule_index`. For an inter-VLAN default-deny posture, place high-index drop rules after the explicit allows.

Rules reference groups by key, not ID: the module resolves `src_group_keys` / `dst_group_keys` against the groups it created, so group and rule ordering is handled internally.

The filipowm/unifi 1.0.0 provider creates resources but cannot update them in place; an edit to an existing group or rule returns "not found". Apply changes by rebuild (destroy and recreate the affected resource).
