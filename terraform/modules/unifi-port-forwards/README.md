# unifi-port-forwards

Creates WAN→LAN port forwards (DNAT) from a data map, one `unifi_port_forward`
per entry. Built for the lab's sanctioned cross-boundary exceptions (e.g. CCTV
cameras on the untrusted house net reaching the NAS), so every forward should be
scoped with `source`.

## Usage

```hcl
module "port_forwards" {
  source = "github.com/lloydoliver/homelab//terraform/modules/unifi-port-forwards?ref=main"

  port_forwards = {
    cctv_nfs = {
      name         = "cctv-nfs"
      protocol     = "tcp"
      wan_port     = "2049"
      forward_ip   = "10.0.20.30" # NAS
      forward_port = "2049"
      source = {
        type              = "firewall-group"
        firewall_group_id = unifi_firewall_group.cameras.id
      }
    }
  }
}
```

## Inputs

`port_forwards` map, per entry: `name`, `wan_port`, `forward_ip`, `forward_port`
required; `protocol` (default `tcp_udp`), `wan_interface` (default `wan`),
`enabled`, `logging`, and an optional `source` block (`type`, `ip`,
`firewall_group_id`) to restrict the source. Lock the source given the hostile
WAN-side network.

## Outputs

- `port_forward_ids` — map of key => UniFi port-forward ID.
