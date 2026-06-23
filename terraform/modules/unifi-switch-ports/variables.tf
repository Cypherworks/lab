variable "switch_mac" {
  description = "MAC of the adopted UniFi switch (the device must already be adopted)."
  type        = string
}

variable "port_profiles" {
  description = <<-EOT
    Reusable named port profiles, keyed by a stable identifier. `forward` sets
    the VLAN mode: all (trunk all), native (access), customize (native + tagged),
    disabled (port off). For customize, `tagged_vlan_mgmt` selects the tagging
    policy and `excluded_network_ids` lists networks to keep off the trunk.
    `poe_mode`: auto | off | pasv24 | passthrough. Set `port_security_enabled`
    with an empty `port_security_macs` to lock a port down (deny all), or list
    allowed MACs to restrict it.
  EOT
  type = map(object({
    name                  = string
    forward               = optional(string)
    native_network_id     = optional(string)
    tagged_vlan_mgmt      = optional(string)
    excluded_network_ids  = optional(set(string))
    poe_mode              = optional(string)
    op_mode               = optional(string, "switch")
    full_duplex           = optional(bool)
    speed                 = optional(number)
    port_security_enabled = optional(bool)
    port_security_macs    = optional(set(string))
  }))
  default = {}
}

variable "ports" {
  description = <<-EOT
    Per-port assignment keyed by port number (as a string). A port references a
    profile by `profile_key`. Set `aggregate_num_ports` to bond this port with
    the following consecutive ports into a LACP aggregate (e.g. 4 = this port +
    next 3). List every port, including a disabled profile for unused ones, for
    full IaC coverage.
  EOT
  type = map(object({
    profile_key         = optional(string)
    name                = optional(string)
    poe_mode            = optional(string)
    aggregate_num_ports = optional(number)
  }))
}

variable "forget_on_destroy" {
  description = "Forget (not factory-reset) the device when the resource is destroyed."
  type        = bool
  default     = true
}
