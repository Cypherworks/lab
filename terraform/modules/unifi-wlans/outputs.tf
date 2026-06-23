output "wlan_ids" {
  description = "Map of WLAN key => UniFi WLAN ID."
  value       = { for k, w in unifi_wlan.this : k => w.id }
}
