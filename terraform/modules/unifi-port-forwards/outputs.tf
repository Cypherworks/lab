output "port_forward_ids" {
  description = "Map of port-forward key => UniFi port-forward ID."
  value       = { for k, p in unifi_port_forward.this : k => p.id }
}
