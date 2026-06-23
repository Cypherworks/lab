terraform {
  required_version = ">= 1.10"
  required_providers {
    unifi = {
      # Pinned to the mature SDKv2 lineage (0.41.x). The 0.52 framework rewrite
      # can't round-trip many unifi_network/port_profile/wlan fields.
      source  = "ubiquiti-community/unifi"
      version = "0.41.5"
    }
  }
}
