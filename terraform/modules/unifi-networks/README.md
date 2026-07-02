# unifi-networks

Creates UniFi corporate networks (VLANs) with optional DHCP, mDNS reflection and isolation from a map of objects.

Part of the [`lab`](https://github.com/Cypherworks/lab) Terraform module collection. Generic and data-driven: pass a map of objects; the module does `for_each` over it. No site data lives here.

## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.10 |
| unifi (filipowm/unifi) | 1.0.0 |

## Inputs

| Name | Type | Default | Required | Description |
|------|------|---------|:--------:|-------------|
| networks | map(object) | n/a | yes | UniFi corporate networks (VLANs) to create, keyed by a stable identifier. |

`networks` object fields:

| Field | Type | Default | Required | Description |
|-------|------|---------|:--------:|-------------|
| name | string | n/a | yes | Network name. |
| vlan_id | number | n/a | yes | VLAN ID. |
| subnet | string | n/a | yes | Gateway IP + prefix, e.g. `10.0.20.1/24`. |
| purpose | string | `"corporate"` | no | UniFi network purpose. |
| dhcp | object | `null` | no | DHCP settings; omit to leave DHCP disabled. |
| multicast_dns | bool | `false` | no | mDNS reflection onto this VLAN. |
| network_isolation | bool | `false` | no | Block intra-VLAN client traffic. |
| internet_access | bool | `true` | no | Allow internet access. |
| ipv6_disabled | bool | `true` | no | Disable IPv6 (sets `ipv6_interface_type = "none"`). |

`dhcp` object fields:

| Field | Type | Default | Required | Description |
|-------|------|---------|:--------:|-------------|
| start | string | n/a | yes | DHCP pool start address. |
| stop | string | n/a | yes | DHCP pool end address. |
| dns | list(string) | `null` | no | DNS servers handed to clients. |
| lease | number | `null` | no | Lease time in seconds. |

## Outputs

| Name | Description |
|------|-------------|
| network_ids | Map of network key => UniFi network ID, for wiring WLANs and firewall rules. |

## Usage

```hcl
module "networks" {
  source = "github.com/Cypherworks/lab//terraform/modules/unifi-networks?ref=<commit-sha>"

  networks = {
    servers = {
      name    = "Servers"
      vlan_id = 20
      subnet  = "10.0.20.1/24"
      dhcp = {
        start = "10.0.20.100"
        stop  = "10.0.20.199"
        dns   = ["10.0.20.10"]
        lease = 86400
      }
    }
    sandbox = {
      name              = "Sandbox"
      vlan_id           = 60
      subnet            = "10.0.60.1/24"
      network_isolation = true
    }
  }
}
```

Pin `ref` to a specific commit SHA or tag, never a moving branch.

## Notes

DHCP is enabled for a network when a `dhcp` object is supplied and disabled when it is omitted. Client DNS servers come from `dhcp.dns`. IPv6 is off by default across the lab.

The filipowm/unifi 1.0.0 provider creates networks but cannot update them in place; an edit to an existing network returns "not found". Apply changes (subnet, DHCP pool, isolation, etc.) by rebuild (destroy and recreate the affected network).
