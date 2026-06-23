# Validation harness with sanitised data. `terraform validate` checks the module
# against the real provider schema without contacting a controller.
terraform {
  required_version = ">= 1.10"
  required_providers {
    unifi = {
      source  = "ubiquiti-community/unifi"
      version = "~> 0.52"
    }
  }
}

provider "unifi" {}

module "wlans" {
  source = "../.."

  wlans = {
    trusted = {
      name          = "example-trusted"
      security      = "wpapsk"
      passphrase    = "changeme-from-sops"
      network_id    = "00000000000000000000000a"
      user_group_id = "00000000000000000000000b"
      ap_group_ids  = ["00000000000000000000000c"]
      wlan_bands    = ["2g", "5g"]
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
