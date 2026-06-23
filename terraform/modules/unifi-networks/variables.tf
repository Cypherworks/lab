variable "networks" {
  description = <<-EOT
    UniFi networks (VLANs) to create, keyed by a stable identifier. 0.41 schema:
    corporate networks with a vlan_id. DHCP name servers come from dhcp.dns (no
    separate enable toggle). IPv6 is disabled by default. mDNS and per-network
    isolation aren't exposed by this provider version, manage those in the
    controller (the USG Pro-4 also caps mDNS at 5 networks).
  EOT
  type = map(object({
    name    = string
    vlan_id = number
    subnet  = string # gateway IP + prefix, e.g. 10.0.20.1/24
    purpose = optional(string, "corporate")
    dhcp = optional(object({
      start = string
      stop  = string
      dns   = optional(list(string)) # DNS servers handed to clients
      lease = optional(number)       # seconds
    }))
    ipv6_disabled = optional(bool, true)
  }))
}
