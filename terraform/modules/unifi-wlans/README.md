# unifi-wlans

Creates UniFi WLANs (SSIDs) from a data map, one `unifi_wlan` per entry.

## Notes

- `user_group_id` is **required** by the provider and there is no user-group data
  source in `ubiquiti-community/unifi`, so the consumer must supply the ID (read
  it from the controller). `ap_group_ids` can be looked up with the
  `unifi_ap_group` data source in the consuming config.
- `network_id` ties the SSID to a VLAN (use the `unifi-networks` module output).
- Passphrases should be supplied from a SOPS-encrypted source, never committed
  in plaintext.

## Usage

```hcl
data "unifi_ap_group" "default" {}

module "wlans" {
  source = "github.com/lloydoliver/homelab//terraform/modules/unifi-wlans?ref=main"

  wlans = {
    trusted = {
      name          = "example-trusted"
      security      = "wpapsk"
      passphrase    = var.trusted_wifi_passphrase # from SOPS
      network_id    = module.networks.network_ids["users"]
      user_group_id = var.default_user_group_id
      ap_group_ids  = [data.unifi_ap_group.default.id]
      wlan_bands    = ["2g", "5g"]
      wpa3_support  = true
    }
  }
}
```

## Outputs

- `wlan_ids` — map of key => UniFi WLAN ID.
