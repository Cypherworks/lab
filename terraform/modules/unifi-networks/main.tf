# Creates one UniFi network (VLAN) per entry in var.networks. Routed by the
# gateway (gateway_type = default). DHCP is optional per network.
resource "unifi_network" "this" {
  for_each = var.networks

  name              = each.value.name
  vlan              = each.value.vlan
  subnet            = each.value.subnet
  gateway_type      = "default"
  domain_name       = each.value.domain_name
  internet_access   = each.value.internet_access
  network_isolation = each.value.network_isolation
  multicast_dns     = each.value.multicast_dns
  igmp_snooping     = each.value.igmp_snooping

  dhcp_server = each.value.dhcp == null ? null : {
    enabled     = each.value.dhcp.enabled
    start       = each.value.dhcp.start
    stop        = each.value.dhcp.stop
    dns_enabled = each.value.dhcp.dns_enabled
    dns_servers = each.value.dhcp.dns_servers
    leasetime   = each.value.dhcp.leasetime
  }
}
