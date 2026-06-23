resource "unifi_firewall_group" "this" {
  for_each = var.firewall_groups

  name    = each.value.name
  type    = each.value.type
  members = each.value.members
}

# Resolves group keys on each rule to the created group IDs.
resource "unifi_firewall_rule" "this" {
  for_each = var.rules

  name       = each.value.name
  ruleset    = each.value.ruleset
  rule_index = each.value.rule_index
  action     = each.value.action
  enabled    = each.value.enabled
  logging    = each.value.logging
  protocol   = each.value.protocol

  src_network_id = each.value.src_network_id
  src_address    = each.value.src_address
  src_port       = each.value.src_port
  src_firewall_group_ids = each.value.src_group_keys == null ? null : [
    for k in each.value.src_group_keys : unifi_firewall_group.this[k].id
  ]

  dst_network_id = each.value.dst_network_id
  dst_address    = each.value.dst_address
  dst_port       = each.value.dst_port
  dst_firewall_group_ids = each.value.dst_group_keys == null ? null : [
    for k in each.value.dst_group_keys : unifi_firewall_group.this[k].id
  ]

  state_established = each.value.state_established
  state_related     = each.value.state_related
  state_invalid     = each.value.state_invalid
  state_new         = each.value.state_new
}
