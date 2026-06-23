# Configures a link-aggregation (LACP) group on an adopted UniFi switch via a
# port override on the leader port. Built for the NAS 4x1GbE bond.
#
# Note: this manages the switch as a unifi_device. Only the listed port override
# is set; verify behaviour against the live switch, as device management depends
# on the device being adopted first.
resource "unifi_device" "switch" {
  mac               = var.switch_mac
  forget_on_destroy = var.forget_on_destroy

  port_override {
    index                  = var.aggregate.leader_index
    op_mode                = "aggregate"
    aggregate_members      = var.aggregate.member_indices
    name                   = var.aggregate.name
    native_networkconf_id  = var.aggregate.native_network_id
    tagged_networkconf_ids = var.aggregate.tagged_network_ids
  }
}
