# unifi-switch-ports

Full-IaC switch configuration: creates reusable port profiles and manages every
port of an adopted UniFi switch as one `unifi_device`, including LACP aggregates.
This is the DR restore point for the switch, the committed config is the truth.

Supersedes the narrower `unifi-switch-lag` module (the aggregate is just one port
entry here). Do not point both at the same switch, they would both manage the
same `unifi_device`.

## Caveats

- The **switch must already be adopted**; supply its MAC.
- LACP aggregates are **consecutive ports**: set `aggregate_num_ports` on the
  lead port (e.g. 4 = lead + next 3). Cable the bonded device to a consecutive
  run.

## Usage

```hcl
module "switch" {
  source = "github.com/Cypherworks/lab//terraform/modules/unifi-switch-ports?ref=main"

  switch_mac = var.switch_mac

  port_profiles = {
    access_servers = { name = "access-servers", forward = "native", native_network_id = net.servers, poe_mode = "auto" }
    trunk_node     = { name = "trunk-node", forward = "customize", native_network_id = net.servers, tagged_vlan_mgmt = "custom", excluded_network_ids = [net.users, net.customer, net.iot] }
    disabled       = { name = "disabled", forward = "disabled" }
  }

  ports = {
    "13" = { profile_key = "trunk_node", name = "thinkcentre-1" }
    "17" = { profile_key = "access_servers", name = "nas-lag", aggregate_num_ports = 4 } # bonds 17-20
    "4"  = { profile_key = "disabled" }
  }
}
```

## Inputs

- `switch_mac` — adopted switch MAC.
- `port_profiles` — named profiles. `forward` = all | native | customize |
  disabled. For `customize`, set `tagged_vlan_mgmt` (auto | block_all | custom)
  and `excluded_network_ids`. Plus `native_network_id`, `poe_mode`, `op_mode`,
  `speed`, `full_duplex`.
- `ports` — keyed by port number (string). `profile_key` assigns a profile; set
  `aggregate_num_ports` on the lead port for a LACP bond. Optional `name`,
  `poe_mode`.

## Outputs

- `port_profile_ids`, `device_id`.
