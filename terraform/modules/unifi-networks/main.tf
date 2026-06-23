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
  # Asserted only when enabling. The provider can't persist false (reads back
  # true → "inconsistent result"), and the controller defaults mDNS on, so
  # disabling it is done in the controller (the USG Pro-4 caps mDNS at 5
  # networks). Left computed when not enabling so Terraform won't fight that.
  multicast_dns = each.value.multicast_dns ? true : null
  igmp_snooping = each.value.igmp_snooping

  ipv6_interface_type = each.value.ipv6_interface_type
  ipv6_ra             = each.value.ipv6_ra

  # dns_enabled is omitted: the provider round-trips it inconsistently
  # (true → false). It's Optional+Computed, so leave it to the controller and
  # set the DHCP name servers via dns_servers.
  dhcp_server = each.value.dhcp == null ? null : {
    enabled     = each.value.dhcp.enabled
    start       = each.value.dhcp.start
    stop        = each.value.dhcp.stop
    dns_servers = each.value.dhcp.dns_servers
    leasetime   = each.value.dhcp.leasetime
  }
}
