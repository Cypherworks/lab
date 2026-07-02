# unifi-wlans

Creates UniFi WLANs (SSIDs) tied to VLANs and user groups from a map of objects.

Part of the [`lab`](https://github.com/Cypherworks/lab) Terraform module collection. Generic and data-driven: pass a map of objects; the module does `for_each` over it. No site data lives here.

## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.10 |
| unifi (filipowm/unifi) | 1.0.0 |

## Inputs

| Name | Type | Default | Required | Description |
|------|------|---------|:--------:|-------------|
| wlans | map(object) | n/a | yes | WLANs (SSIDs) to create, keyed by a stable identifier. |

`wlans` object fields:

| Field | Type | Default | Required | Description |
|-------|------|---------|:--------:|-------------|
| name | string | n/a | yes | SSID name. |
| user_group_id | string | n/a | yes | References a `unifi_user_group`. |
| network_id | string | `null` | no | Ties the SSID to a VLAN. |
| ap_group_ids | set(string) | `null` | no | AP groups the SSID broadcasts on. |
| security | string | `"wpapsk"` | no | Security mode. |
| passphrase | string | `null` | no | PSK passphrase; source from SOPS, not plaintext. |
| wlan_band | string | `"both"` | no | `both` \| `2g` \| `5g`. |
| is_guest | bool | `false` | no | Mark as a guest network. |
| hide_ssid | bool | `false` | no | Hide the SSID. |
| l2_isolation | bool | `false` | no | Layer-2 client isolation. |
| wpa3_support | bool | `false` | no | Enable WPA3. |
| wpa3_transition | bool | `false` | no | WPA3 transition mode. |
| pmf_mode | string | `null` | no | Protected Management Frames mode. |

## Outputs

| Name | Description |
|------|-------------|
| wlan_ids | Map of WLAN key => UniFi WLAN ID. |

## Usage

```hcl
module "wlans" {
  source = "github.com/Cypherworks/lab//terraform/modules/unifi-wlans?ref=<commit-sha>"

  wlans = {
    trusted = {
      name          = "example-trusted"
      security      = "wpapsk"
      passphrase    = "changeme-from-sops"
      network_id    = "00000000000000000000000a"
      user_group_id = "00000000000000000000000b"
      ap_group_ids  = ["00000000000000000000000c"]
      wlan_band     = "both"
      wpa3_support  = true
    }
    customer = {
      name          = "example-customer"
      security      = "wpapsk"
      passphrase    = "changeme-from-sops"
      network_id    = "00000000000000000000000d"
      user_group_id = "00000000000000000000000b"
      is_guest      = true
    }
  }
}
```

Pin `ref` to a specific commit SHA or tag, never a moving branch.

## Notes

`network_id` ties the SSID to a VLAN and `user_group_id` references a `unifi_user_group`; both come from the consuming configuration, not this module.

Passphrases should come from a SOPS-encrypted source, never committed in plaintext.
