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

module "firewall" {
  source = "../.."

  firewall_groups = {
    cameras = {
      name    = "cctv-cameras"
      type    = "address-group"
      members = ["192.0.2.10", "192.0.2.11"]
    }
  }

  rules = {
    allow_cameras_to_nas = {
      name           = "cameras-to-nas-nfs"
      ruleset        = "LAN_IN"
      rule_index     = 2010
      action         = "accept"
      protocol       = "tcp"
      src_group_keys = ["cameras"]
      dst_address    = "10.0.20.30"
      dst_port       = "2049"
    }
    drop_inter_vlan = {
      name        = "drop-inter-vlan"
      ruleset     = "LAN_IN"
      rule_index  = 2999
      action      = "drop"
      src_address = "10.0.0.0/8"
      dst_address = "10.0.0.0/8"
    }
  }
}
