variable "port_forwards" {
  description = <<-EOT
    WAN→LAN port forwards (DNAT), keyed by a stable identifier. `src_ip` restricts
    the source (a single IP or CIDR, e.g. the two cameras as 192.168.178.10/31);
    omit for unrestricted. The source net is hostile, so scope inbound forwards.
  EOT
  type = map(object({
    name         = string
    protocol     = optional(string, "tcp_udp") # tcp | udp | tcp_udp
    interface    = optional(string, "wan")     # port_forward_interface
    wan_port     = string                      # external (dst) port
    forward_ip   = string                      # internal target IP
    forward_port = string                      # internal target port
    src_ip       = optional(string)            # source IP/CIDR restriction
    log          = optional(bool, false)
  }))
}
