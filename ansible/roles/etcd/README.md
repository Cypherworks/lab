# etcd

Pinned etcd cluster — the Patroni DCS (quorum) for the Authentik HA data tier.
Each member's bind address is its entry in `etcd_members`, matched by
`inventory_hostname`, so the multi-homed Pi witness binds its VLAN-30 foot
(10.200.30.13) rather than its VLAN-20 DNS address.

## Required inventory vars

```yaml
etcd_members:
  - { name: etcd-tc1, ip: 10.200.30.31 }
  - { name: etcd-tc2, ip: 10.200.30.32 }
  - { name: pi-dns-1, ip: 10.200.30.13 }   # the Pi's VLAN-30 sub-interface
```

`name` must equal the host's `inventory_hostname`. Quorum is a majority (2 of 3).

## ⚠️ Security follow-up — TLS

v1 runs **plain HTTP** for peer and client traffic. It's intra-VLAN-30 only, but
this cluster carries the Postgres leader state, so **add peer + client TLS** (a
small CA + per-member certs) before treating it as fully load-bearing.

## Re-adding a member

`initial-cluster-state: new` bootstraps a fresh cluster. To re-add a wiped member
to a *live* cluster, register it (`etcdctl member add`) and set
`etcd_initial_cluster_state: existing` for that host only, then revert.
