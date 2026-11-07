# Terraform modules

Reusable, parameterised Terraform modules for a UniFi lab. Each module is a generic mechanism: it takes a map of objects and does `for_each` over it. No site data lives here. A private deploy repo consumes these modules and supplies all site values (network IDs, MACs, passphrases, rule tables).

Consume a module by Git source, pinned to a commit SHA or tag:

```hcl
module "networks" {
  source = "github.com/Cypherworks/lab//terraform/modules/unifi-networks?ref=<commit-sha>"
  # ...
}
```

Always pin `ref` to a specific commit SHA or tag, never a moving branch.

## Provider

All modules require `terraform >= 1.10` and provider `filipowm/unifi` version `1.0.0`.

filipowm/unifi 1.0.0 was chosen over the ubiquiti-community forks because those forks could not model the lab: 0.41.x cannot set `vlan_enabled`, so it cannot create VLANs, and 0.41.25 / 0.52.x could not round-trip port forwards, tagged-VLAN port config, or multicast DNS. filipowm 1.0.0 handles all of these against controller 8.6.9.

Operational constraint: filipowm 1.0.0 applies an in-place edit that sets or changes a value. That was tested against controller 8.6.9 for networks, firewall groups and rules, port profiles, clients and user groups; the switch device, WLANs and port forwards use the same update code but weren't tested. It can't clear an attribute: go-unifi leaves empty strings and lists out of the update, the controller keeps the stored value, and its empty reply surfaces as "not found". Clear an attribute by recreating the resource (`terraform apply -replace=<address>`), and plan for a brief disruption when that resource is a network, port profile, or firewall rule.

## Modules

| Module | Purpose |
|--------|---------|
| [unifi-networks](unifi-networks/) | Corporate networks (VLANs) with optional DHCP, mDNS reflection and isolation. |
| [unifi-wlans](unifi-wlans/) | WLANs (SSIDs) tied to VLANs and user groups. |
| [unifi-firewall](unifi-firewall/) | Firewall address/port groups and legacy USG rules. |
| [unifi-port-forwards](unifi-port-forwards/) | WAN→LAN port forwards (DNAT) with source restriction. |
| [unifi-switch-ports](unifi-switch-ports/) | Adopted-switch port profiles and per-port overrides, including LACP aggregates. Supersedes the former `unifi-switch-lag` module (an aggregate is one port entry with `aggregate_num_ports`). |
