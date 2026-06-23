# Creates one UniFi corporate network (VLAN) per entry in var.networks.
resource "unifi_network" "this" {
  for_each = var.networks

  name    = each.value.name
  purpose = each.value.purpose
  vlan_id = each.value.vlan_id
  subnet  = each.value.subnet

  dhcp_enabled = each.value.dhcp != null
  dhcp_start   = try(each.value.dhcp.start, null)
  dhcp_stop    = try(each.value.dhcp.stop, null)
  dhcp_dns     = try(each.value.dhcp.dns, null)
  dhcp_lease   = try(each.value.dhcp.lease, null)

  # IPv6 off across the lab.
  ipv6_interface_type = each.value.ipv6_disabled ? "none" : null
}
