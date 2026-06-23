# Creates one WLAN (SSID) per entry in var.wlans.
resource "unifi_wlan" "this" {
  for_each = var.wlans

  name            = each.value.name
  security        = each.value.security
  passphrase      = each.value.passphrase
  user_group_id   = each.value.user_group_id
  network_id      = each.value.network_id
  ap_group_ids    = each.value.ap_group_ids
  wlan_bands      = each.value.wlan_bands
  is_guest        = each.value.is_guest
  hide_ssid       = each.value.hide_ssid
  l2_isolation    = each.value.l2_isolation
  wpa3_support    = each.value.wpa3_support
  wpa3_transition = each.value.wpa3_transition
  pmf_mode        = each.value.pmf_mode
  enabled         = each.value.enabled
}
