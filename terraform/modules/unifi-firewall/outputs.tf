output "firewall_group_ids" {
  description = "Map of group key => UniFi firewall group ID."
  value       = { for k, g in unifi_firewall_group.this : k => g.id }
}

output "firewall_rule_ids" {
  description = "Map of rule key => UniFi firewall rule ID."
  value       = { for k, r in unifi_firewall_rule.this : k => r.id }
}
