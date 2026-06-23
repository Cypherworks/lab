resource "unifi_port_profile" "this" {
  for_each = var.port_profiles

  name                   = each.value.name
  forward                = each.value.forward
  native_networkconf_id  = each.value.native_network_id
  tagged_networkconf_ids = each.value.tagged_network_ids
  poe_mode               = each.value.poe_mode
  op_mode                = each.value.op_mode
  full_duplex            = each.value.full_duplex
  speed                  = each.value.speed
}

# Manages the whole switch device: one port override per entry in var.ports.
# Ports reference a named profile by key; an aggregate port sets op_mode inline.
resource "unifi_device" "switch" {
  mac               = var.switch_mac
  forget_on_destroy = var.forget_on_destroy

  dynamic "port_override" {
    for_each = var.ports
    content {
      number                = tonumber(port_override.key)
      name                  = port_override.value.name
      native_networkconf_id = port_override.value.native_network_id
      port_profile_id       = port_override.value.profile_key != null ? unifi_port_profile.this[port_override.value.profile_key].id : null
      op_mode               = port_override.value.aggregate != null ? "aggregate" : null
      aggregate_members     = port_override.value.aggregate != null ? port_override.value.aggregate.members : null
    }
  }
}
