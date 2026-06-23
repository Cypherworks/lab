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

module "port_forwards" {
  source = "../.."

  port_forwards = {
    cctv_ftp = {
      name         = "cctv-ftp"
      protocol     = "tcp"
      wan_port     = "21"
      forward_ip   = "10.0.20.30"
      forward_port = "21"
      src_ip       = "192.0.2.10/31" # two cameras
    }
  }
}
