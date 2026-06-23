# unifi-switch-lag

Configures an LACP link-aggregation group on an adopted UniFi switch, for the
NAS 4x1GbE bond. The leader port is set to `op_mode = aggregate` and lists the
member port indices; native/tagged network IDs set the VLANs on the bond.

## Caveats

- Manages the switch as a `unifi_device`, so the **switch must already be
  adopted** and you must supply its MAC.
- Only the specified port override is managed here.
- Aggregate semantics (which port is the leader, how members are listed) should
  be **verified against the live switch** — this is the most hardware-dependent
  module in the set.

## Usage

```hcl
module "nas_lag" {
  source = "github.com/lloydoliver/homelab//terraform/modules/unifi-switch-lag?ref=main"

  switch_mac = var.switch_mac
  aggregate = {
    leader_index      = 10
    member_indices    = [10, 11, 12, 13] # NAS 4x1GbE
    name              = "nas-bond"
    native_network_id = module.networks.network_ids["servers"]
  }
}
```

## Outputs

- `device_id` — ID of the managed switch device.
