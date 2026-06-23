output "network_ids" {
  description = "Map of network key => UniFi network ID, for wiring WLANs and firewall rules."
  value       = { for k, n in unifi_network.this : k => n.id }
}
