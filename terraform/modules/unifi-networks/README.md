# unifi-networks

Creates UniFi networks (VLANs) from a data map. Generic mechanism: the consuming
deployment supplies the VLAN definitions; this module just maps them to
`unifi_network` resources, one per entry, routed by the gateway.

## Provider

Uses [`filipowm/unifi`](https://registry.terraform.io/providers/filipowm/unifi)
(`1.0.0`). It creates VLANs correctly, round-trips cleanly, and supports
controller 6.x+. The ubiquiti-community forks were dropped: early 0.41.x can't
set `vlan_enabled` (VLAN creates fail), and 0.41.25/0.52.x can't round-trip
`forward`/tagged-VLAN/`multicast_dns`.

## Usage

```hcl
module "networks" {
  source = "github.com/Cypherworks/lab//terraform/modules/unifi-networks?ref=main"

  networks = {
    servers = {
      name   = "Servers"
      vlan   = 20
      subnet = "10.0.20.1/24"
      dhcp   = { start = "10.0.20.100", stop = "10.0.20.199" }
    }
    iot = {
      name              = "IoT"
      vlan              = 50
      subnet            = "10.0.50.1/24"
      network_isolation = true
      multicast_dns     = true
    }
  }
}
```

## Inputs

`networks` is a map keyed by a stable identifier. Per entry:

| Field | Required | Default | Notes |
|---|---|---|---|
| `name` | yes | | Display name in the controller |
| `vlan` | yes | | VLAN ID |
| `subnet` | yes | | Gateway IP + prefix, CIDR (e.g. `10.0.20.1/24`) |
| `domain_name` | no | | Search domain |
| `internet_access` | no | `true` | |
| `network_isolation` | no | `false` | Block inter-client/inter-VLAN routing |
| `multicast_dns` | no | `false` | mDNS reflection for this network |
| `igmp_snooping` | no | `false` | |
| `dhcp` | no | | Omit for no DHCP server. `start`/`stop` required when set; `dns_servers`, `leasetime`, `dns_enabled`, `enabled` optional |

## Outputs

- `network_ids` — map of key => UniFi network ID, for wiring WLANs/firewall rules.
