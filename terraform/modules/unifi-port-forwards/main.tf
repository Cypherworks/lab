# Creates one WAN→LAN port forward (DNAT) per entry in var.port_forwards.
resource "unifi_port_forward" "this" {
  for_each = var.port_forwards

  name     = each.value.name
  protocol = each.value.protocol
  enabled  = each.value.enabled
  logging  = each.value.logging

  wan = {
    interface = each.value.wan_interface
    port      = each.value.wan_port
  }

  forward = {
    ip   = each.value.forward_ip
    port = each.value.forward_port
  }

  source_limiting = each.value.source == null ? null : {
    enabled           = each.value.source.enabled
    type              = each.value.source.type
    ip                = each.value.source.ip
    firewall_group_id = each.value.source.firewall_group_id
  }
}
