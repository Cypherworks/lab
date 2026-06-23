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

module "port_forwards" {
  source = "../.."

  port_forwards = {
    cctv_nfs = {
      name         = "cctv-nfs"
      protocol     = "tcp"
      wan_port     = "2049"
      forward_ip   = "10.0.20.30"
      forward_port = "2049"
      source = {
        type = "address"
        ip   = "192.0.2.10"
      }
    }
  }
}
