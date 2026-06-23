# Creates one WAN→LAN port forward (DNAT) per entry. 0.41 uses flat fields and a
# single src_ip for source restriction (use a CIDR to cover a small range, e.g.
# the two cameras as a /31).
resource "unifi_port_forward" "this" {
  for_each = var.port_forwards

  name                   = each.value.name
  protocol               = each.value.protocol
  port_forward_interface = each.value.interface
  dst_port               = each.value.wan_port
  fwd_ip                 = each.value.forward_ip
  fwd_port               = each.value.forward_port
  src_ip                 = each.value.src_ip
  log                    = each.value.log
}
