variable "port_forwards" {
  description = <<-EOT
    WAN→LAN port forwards (DNAT), keyed by a stable identifier. Set `source` to
    restrict by source address or a firewall group (e.g. lock the CCTV forward to
    the camera IPs). Source net is hostile, so always scope inbound forwards.
  EOT
  type = map(object({
    name          = string
    protocol      = optional(string, "tcp_udp") # tcp | udp | tcp_udp
    enabled       = optional(bool, true)
    logging       = optional(bool, false)
    wan_interface = optional(string, "wan")
    wan_port      = string # external port on the WAN
    forward_ip    = string # internal target IP
    forward_port  = string # internal target port
    source = optional(object({
      enabled           = optional(bool, true)
      type              = optional(string) # e.g. address | firewall-group
      ip                = optional(string)
      firewall_group_id = optional(string)
    }))
  }))
}
