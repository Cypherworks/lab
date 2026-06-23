variable "switch_mac" {
  description = "MAC of the adopted UniFi switch to configure (the device must already be adopted)."
  type        = string
}

variable "aggregate" {
  description = <<-EOT
    A link aggregation (LACP) group on the switch. The leader port carries
    op_mode=aggregate and lists the member port indices. Used for the NAS
    4x1GbE bond. native/tagged network IDs set the VLANs on the aggregate.
  EOT
  type = object({
    leader_index       = number
    member_indices     = list(number)
    name               = optional(string, "LAG")
    native_network_id  = optional(string)
    tagged_network_ids = optional(set(string))
  })
}

variable "forget_on_destroy" {
  description = "Whether to forget (not factory-reset) the device when the resource is destroyed."
  type        = bool
  default     = true
}
