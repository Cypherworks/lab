# Validation harness with sanitised data.
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

module "nas_lag" {
  source = "../.."

  switch_mac = "00:11:22:33:44:55"
  aggregate = {
    leader_index      = 10
    member_indices    = [10, 11, 12, 13]
    name              = "nas-bond"
    native_network_id = "00000000000000000000000a"
  }
}
