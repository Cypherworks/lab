# unifi-firewall

Creates legacy USG firewall groups and rules from data maps. Rules reference
groups created in the same module by key, so the consumer never has to wire group
IDs by hand.

The USG Pro-4 on a self-hosted controller uses the legacy ruleset model
(`LAN_IN`, `LAN_LOCAL`, `GUEST_IN`, `WAN_IN`, ...) ordered by `rule_index`, not
the newer zone-based policies. For an inter-VLAN **default-deny** posture: allow
established/related, add the explicit inter-VLAN/service allows, then a high-index
drop covering RFC1918→RFC1918 in `LAN_IN`.

## Usage

```hcl
module "firewall" {
  source = "github.com/Cypherworks/lab//terraform/modules/unifi-firewall?ref=main"

  firewall_groups = {
    cameras = { name = "cctv-cameras", type = "address-group", members = ["192.0.2.10", "192.0.2.11"] }
  }

  rules = {
    allow_macbook_to_printer = {
      name           = "macbook-to-3dprinter"
      ruleset        = "LAN_IN"
      rule_index     = 2010
      action         = "accept"
      src_address    = "10.0.40.20"
      dst_address    = "10.0.50.60"
    }
    drop_inter_vlan = {
      name           = "drop-inter-vlan"
      ruleset        = "LAN_IN"
      rule_index     = 2999
      action         = "drop"
      src_address    = "10.0.0.0/8"
      dst_address    = "10.0.0.0/8"
    }
  }
}
```

## Inputs

- `firewall_groups` — map of `{ name, type, members }`. `type` e.g.
  `address-group`, `port-group`.
- `rules` — map of rules. `name`, `ruleset`, `rule_index`, `action` required;
  optional `protocol`, `src_*`/`dst_*` (address, network_id, port), and
  `src_group_keys`/`dst_group_keys` (keys into `firewall_groups`), plus
  connection-state matches. Indices must fall in the ruleset's valid range.

## Outputs

- `firewall_group_ids`, `firewall_rule_ids` — key => UniFi ID maps.
