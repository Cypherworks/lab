output "port_profile_ids" {
  description = "Map of profile key => UniFi port-profile ID."
  value       = { for k, p in unifi_port_profile.this : k => p.id }
}

output "device_id" {
  description = "ID of the managed switch device."
  value       = unifi_device.switch.id
}
