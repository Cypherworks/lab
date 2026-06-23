variable "switch_mac" {
  description = "MAC of the adopted UniFi switch (the device must already be adopted)."
  type        = string
}

variable "port_profiles" {
  description = <<-EOT
    Reusable named port profiles, keyed by a stable identifier. `forward` sets
    the VLAN mode: all (trunk all), native (access), customize (native + tagged),
    disabled (port off). `poe_mode`: auto | off | pasv24 | passthrough.
  EOT
  type = map(object({
    name               = string
    forward            = optional(string)
    native_network_id  = optional(string)
    tagged_network_ids = optional(set(string))
    poe_mode           = optional(string)
    op_mode            = optional(string, "switch")
    full_duplex        = optional(bool)
    speed              = optional(number)
  }))
  default = {}
}

variable "ports" {
  description = <<-EOT
    Per-port assignment keyed by port index (as a string). A port either
    references a profile by `profile_key`, or defines an inline `aggregate`
    (LACP) whose `members` are the bonded port indices. List every port,
    including a disabled profile for unused ones, for full IaC coverage.
  EOT
  type = map(object({
    profile_key       = optional(string)
    name              = optional(string)
    native_network_id = optional(string) # for aggregate ports
    aggregate = optional(object({
      members = list(number)
    }))
  }))
}

variable "forget_on_destroy" {
  description = "Forget (not factory-reset) the device when the resource is destroyed."
  type        = bool
  default     = true
}
