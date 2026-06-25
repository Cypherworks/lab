# haproxy_patroni

Local HAProxy for the Authentik **app** nodes. Binds `127.0.0.1:5000` and forwards
to whichever Postgres node is the current Patroni primary, using Patroni's REST
`/primary` healthcheck (returns 200 only on the leader, so a demoted primary is
marked down and its sessions dropped). Authentik connects to `127.0.0.1:5000`.

## Required inventory vars

```yaml
patroni_members:
  - { name: pg-tc1, ip: 10.200.30.33 }
  - { name: pg-tc2, ip: 10.200.30.34 }
```

Run this on each app node alongside the Authentik server/worker.
