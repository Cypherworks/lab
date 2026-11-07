resource "unifi_port_profile" "this" {
  for_each = var.port_profiles

  name                  = each.value.name
  forward               = each.value.forward
  native_networkconf_id = each.value.native_network_id
  tagged_vlan_mgmt      = each.value.tagged_vlan_mgmt
  excluded_network_ids  = each.value.excluded_network_ids
  poe_mode              = each.value.poe_mode
  op_mode               = each.value.op_mode
  full_duplex           = each.value.full_duplex
  speed                 = each.value.speed

  port_security_enabled     = each.value.port_security_enabled
  port_security_mac_address = each.value.port_security_macs

  # Both are controller-derived; an update of either fails as "not found".
  lifecycle {
    ignore_changes = [forward, tagged_vlan_mgmt]
  }
}

# Manages the whole switch device: one port override per entry in var.ports.
# Ports reference a named profile by key; an aggregate port bonds the following
# consecutive ports via aggregate_num_ports.
resource "unifi_device" "switch" {
  mac               = var.switch_mac
  forget_on_destroy = var.forget_on_destroy

  dynamic "port_override" {
    for_each = var.ports
    content {
      number              = tonumber(port_override.key)
      name                = port_override.value.name
      port_profile_id     = port_override.value.profile_key != null ? unifi_port_profile.this[port_override.value.profile_key].id : null
      poe_mode            = port_override.value.poe_mode
      op_mode             = port_override.value.aggregate_num_ports != null ? "aggregate" : null
      aggregate_num_ports = port_override.value.aggregate_num_ports
    }
  }
}
