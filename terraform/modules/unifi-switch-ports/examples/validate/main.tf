# Validation harness mirroring the lab switch layout with sanitised network IDs.
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

locals {
  net = {
    mgmt     = "000000000000000000000010"
    servers  = "000000000000000000000020"
    services = "000000000000000000000030"
    users    = "000000000000000000000040"
    customer = "000000000000000000000045"
    iot      = "000000000000000000000050"
    sandbox  = "000000000000000000000060"
  }
}

module "switch" {
  source     = "../.."
  switch_mac = "00:11:22:33:44:55"

  port_profiles = {
    uplink_all     = { name = "uplink-all", forward = "all", native_network_id = local.net.mgmt, poe_mode = "off" }
    trunk_ap       = { name = "trunk-ap", forward = "customize", native_network_id = local.net.mgmt, tagged_network_ids = [local.net.users, local.net.customer, local.net.iot], poe_mode = "auto" }
    access_servers = { name = "access-servers", forward = "native", native_network_id = local.net.servers, poe_mode = "auto" }
    access_iot     = { name = "access-iot", forward = "native", native_network_id = local.net.iot, poe_mode = "off" }
    trunk_node     = { name = "trunk-node", forward = "customize", native_network_id = local.net.servers, tagged_network_ids = [local.net.mgmt, local.net.services, local.net.sandbox], poe_mode = "off" }
    disabled       = { name = "disabled", forward = "disabled" }
  }

  ports = {
    "1"  = { profile_key = "access_iot", name = "soundtouch" }
    "2"  = { profile_key = "uplink_all", name = "usg" }
    "3"  = { profile_key = "trunk_ap", name = "ap-ac-pro" }
    "5"  = { profile_key = "access_servers", name = "pi-dns-1" }
    "7"  = { profile_key = "access_servers", name = "pi-dns-2" }
    "9"  = { profile_key = "access_iot", name = "pi-kiosk" }
    "11" = { profile_key = "trunk_node", name = "ryzen" }
    "13" = { profile_key = "trunk_node", name = "thinkcentre-1" }
    "15" = { profile_key = "trunk_node", name = "thinkcentre-2" }
    "17" = { name = "nas-lag", native_network_id = local.net.servers, aggregate = { members = [17, 19, 21, 23] } }
    # Unused ports (and SFP 25/26) shut for zero-trust hygiene.
    "4"  = { profile_key = "disabled" }
    "6"  = { profile_key = "disabled" }
    "8"  = { profile_key = "disabled" }
    "10" = { profile_key = "disabled" }
    "12" = { profile_key = "disabled" }
    "14" = { profile_key = "disabled" }
    "16" = { profile_key = "disabled" }
    "18" = { profile_key = "disabled" }
    "20" = { profile_key = "disabled" }
    "22" = { profile_key = "disabled" }
    "24" = { profile_key = "disabled" }
    "25" = { profile_key = "disabled" }
    "26" = { profile_key = "disabled" }
  }
}
