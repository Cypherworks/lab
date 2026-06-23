# Validation harness with sanitised data.
terraform {
  required_version = ">= 1.10"
  required_providers {
    unifi = {
      source  = "ubiquiti-community/unifi"
      version = "0.41.5"
    }
  }
}

provider "unifi" {}

module "networks" {
  source = "../.."

  networks = {
    servers = {
      name    = "Servers"
      vlan_id = 20
      subnet  = "10.0.20.1/24"
      dhcp = {
        start = "10.0.20.100"
        stop  = "10.0.20.199"
        dns   = ["10.0.20.10"]
        lease = 86400
      }
    }
    sandbox = {
      name    = "Sandbox"
      vlan_id = 60
      subnet  = "10.0.60.1/24"
    }
  }
}
