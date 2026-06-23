variable "firewall_groups" {
  description = <<-EOT
    Address/port groups, keyed by a stable identifier. Rules reference these by
    key (see `rules`), so the module resolves keys to IDs internally.
    `type` is e.g. address-group, port-group, ipv6-address-group.
  EOT
  type = map(object({
    name    = string
    type    = string
    members = list(string)
  }))
  default = {}
}

variable "rules" {
  description = <<-EOT
    Legacy USG firewall rules, keyed by a stable identifier. The USG Pro-4 on a
    self-hosted controller uses rulesets (LAN_IN, LAN_LOCAL, GUEST_IN, WAN_IN,
    etc.) ordered by rule_index. Reference groups created in this module via
    src_group_keys / dst_group_keys. For an inter-VLAN default-deny posture,
    place high-index drop rules after the explicit allows.
  EOT
  type = map(object({
    name       = string
    ruleset    = string
    rule_index = number
    action     = string # accept | drop | reject
    enabled    = optional(bool, true)
    logging    = optional(bool, false)
    protocol   = optional(string)

    src_network_id = optional(string)
    src_address    = optional(string)
    src_group_keys = optional(list(string)) # keys into firewall_groups
    src_port       = optional(string)

    dst_network_id = optional(string)
    dst_address    = optional(string)
    dst_group_keys = optional(list(string)) # keys into firewall_groups
    dst_port       = optional(string)

    state_established = optional(bool)
    state_related     = optional(bool)
    state_invalid     = optional(bool)
    state_new         = optional(bool)
  }))
}
