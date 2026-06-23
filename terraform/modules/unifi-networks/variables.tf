variable "networks" {
  description = "UniFi networks (VLANs) to create, keyed by a stable identifier."
  type = map(object({
    name              = string
    vlan              = number
    subnet            = string # gateway IP + prefix, e.g. 10.0.10.1/24
    domain_name       = optional(string)
    internet_access   = optional(bool, true)
    network_isolation = optional(bool, false)
    multicast_dns     = optional(bool, false)
    igmp_snooping     = optional(bool, false)
    dhcp = optional(object({
      enabled     = optional(bool, true)
      start       = string
      stop        = string
      dns_enabled = optional(bool, true)
      dns_servers = optional(list(string))
      leasetime   = optional(string)
    }))
  }))
}
