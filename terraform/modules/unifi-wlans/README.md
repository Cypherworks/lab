# unifi-wlans

Creates UniFi WLANs (SSIDs) from a data map, one `unifi_wlan` per entry.

## Notes

- `user_group_id` is **required**; create one with the `unifi_user_group`
  resource (filipowm provides it) and pass its ID. `ap_group_ids` can be looked
  up with the `unifi_ap_group` data source in the consuming config.
- `wlan_band` is a single value: `both` (default), `2g`, or `5g`.
- `network_id` ties the SSID to a VLAN (use the `unifi-networks` module output).
- Passphrases should be supplied from a SOPS-encrypted source, never committed
  in plaintext.

## Usage

```hcl
data "unifi_ap_group" "default" {}

resource "unifi_user_group" "default" {
  name = "lab"
}

module "wlans" {
  source = "github.com/Cypherworks/lab//terraform/modules/unifi-wlans?ref=main"

  wlans = {
    trusted = {
      name          = "example-trusted"
      security      = "wpapsk"
      passphrase    = var.trusted_wifi_passphrase # from SOPS
      network_id    = module.networks.network_ids["users"]
      user_group_id = unifi_user_group.default.id
      ap_group_ids  = [data.unifi_ap_group.default.id]
      wlan_band     = "both"
      wpa3_support  = true
    }
  }
}
```

## Outputs

- `wlan_ids` — map of key => UniFi WLAN ID.
