# unifi-switch-ports

Full-IaC switch configuration: creates reusable port profiles and manages every
port of an adopted UniFi switch as one `unifi_device`, including LACP aggregates.
This is the DR restore point for the switch, the committed config is the truth.

Supersedes the narrower `unifi-switch-lag` module (the aggregate is just one port
entry here). Do not point both at the same switch, they would both manage the
same `unifi_device`.

## Caveats

- The **switch must already be adopted**; supply its MAC.
- Aggregate semantics (lead port + members) should be verified on the live
  switch; this provider/device area is the most hardware-dependent.

## Usage

```hcl
module "switch" {
  source = "github.com/lloydoliver/homelab//terraform/modules/unifi-switch-ports?ref=main"

  switch_mac = var.switch_mac

  port_profiles = {
    access_servers = { name = "access-servers", forward = "native", native_network_id = net.servers, poe_mode = "auto" }
    trunk_node     = { name = "trunk-node", forward = "customize", native_network_id = net.servers, tagged_network_ids = [net.mgmt, net.services, net.sandbox] }
    disabled       = { name = "disabled", forward = "disabled" }
  }

  ports = {
    "13" = { profile_key = "trunk_node", name = "thinkcentre-1" }
    "17" = { name = "nas-lag", native_network_id = net.servers, aggregate = { members = [17, 19, 21, 23] } }
    "4"  = { profile_key = "disabled" }
  }
}
```

## Inputs

- `switch_mac` — adopted switch MAC.
- `port_profiles` — named profiles. `forward` = all | native | customize |
  disabled; plus `native_network_id`, `tagged_network_ids`, `poe_mode`,
  `op_mode`, `speed`, `full_duplex`.
- `ports` — keyed by port index (string). Either `profile_key`, or an inline
  `aggregate { members }`. Optional `name`, `native_network_id`.

## Outputs

- `port_profile_ids`, `device_id`.
