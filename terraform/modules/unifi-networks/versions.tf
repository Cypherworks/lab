terraform {
  required_version = ">= 1.10"
  required_providers {
    unifi = {
      # paultyng/unifi was archived 2026-04-30; ubiquiti-community is the
      # maintained successor. Pinned to the 0.52.x line (modern framework schema).
      source  = "ubiquiti-community/unifi"
      version = "~> 0.52"
    }
  }
}
