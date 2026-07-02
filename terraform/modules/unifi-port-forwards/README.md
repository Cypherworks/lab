# unifi-port-forwards

Creates WAN→LAN port forwards (DNAT) from a map of objects.

Part of the [`lab`](https://github.com/Cypherworks/lab) Terraform module collection. Generic and data-driven: pass a map of objects; the module does `for_each` over it. No site data lives here.

## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.10 |
| unifi (filipowm/unifi) | 1.0.0 |

## Inputs

| Name | Type | Default | Required | Description |
|------|------|---------|:--------:|-------------|
| port_forwards | map(object) | n/a | yes | WAN→LAN port forwards (DNAT) keyed by a stable identifier. |

`port_forwards` object fields:

| Field | Type | Default | Required | Description |
|-------|------|---------|:--------:|-------------|
| name | string | n/a | yes | Forward name. |
| protocol | string | `"tcp_udp"` | no | `tcp` \| `udp` \| `tcp_udp`. |
| interface | string | `"wan"` | no | Port-forward interface. |
| wan_port | string | n/a | yes | External (destination) port. |
| forward_ip | string | n/a | yes | Internal target IP. |
| forward_port | string | n/a | yes | Internal target port. |
| src_ip | string | `null` | no | Source restriction: a single IP or CIDR; omit for unrestricted. |
| log | bool | `false` | no | Log matches. |

## Outputs

| Name | Description |
|------|-------------|
| port_forward_ids | Map of port-forward key => UniFi port-forward ID. |

## Usage

```hcl
module "port_forwards" {
  source = "github.com/Cypherworks/lab//terraform/modules/unifi-port-forwards?ref=<commit-sha>"

  port_forwards = {
    cctv_ftp = {
      name         = "cctv-ftp"
      protocol     = "tcp"
      wan_port     = "21"
      forward_ip   = "10.0.20.30"
      forward_port = "21"
      src_ip       = "192.0.2.10/31" # two cameras
    }
  }
}
```

Pin `ref` to a specific commit SHA or tag, never a moving branch.

## Notes

`src_ip` is a single flat field (one IP or CIDR). To cover a small range, use a CIDR, e.g. two adjacent camera addresses as a `/31`. The WAN side is hostile, so scope inbound forwards rather than leaving `src_ip` unset.
