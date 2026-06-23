# Validation harness with sanitised example data. `terraform validate` here
# checks the module against the real provider schema without contacting a
# controller. Not a live config — the real VLAN data lives in homelab-deploy.
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

module "networks" {
  source = "../.."

  networks = {
    servers = {
      name        = "Servers"
      vlan        = 20
      subnet      = "10.0.20.1/24"
      domain_name = "example.test"
      dhcp = {
        start       = "10.0.20.100"
        stop        = "10.0.20.199"
        dns_servers = ["10.0.20.10"]
        leasetime   = "86400"
      }
    }
    iot = {
      name              = "IoT"
      vlan              = 50
      subnet            = "10.0.50.1/24"
      network_isolation = true
      multicast_dns     = true
    }
  }
}
