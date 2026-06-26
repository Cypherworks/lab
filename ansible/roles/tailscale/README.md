# tailscale

Joins a host to the lab's Headscale overlay with a pre-auth key.

Used to put the **Headscale EC2 on its own tailnet** so the `sni_router` can reach
the lab Caddy (`10.200.30.10`) over the overlay for the `auth.cypherworks.co.uk`
passthrough. `--accept-routes` pulls in the `10.200.0.0/16` advertised by the HA
subnet routers.

Ordering note: the EC2 joins the very Headscale it hosts, so Headscale **and** the
SNI router must be up first (the client dials `https://headscale.cypherworks.co.uk`
→ EIP:443 → nginx → Headscale). Idempotent: only runs `tailscale up` if the
backend isn't already `Running`.

| Variable | Purpose |
| --- | --- |
| `tailscale_login_server` | Headscale URL to join |
| `tailscale_authkey` | short-lived pre-auth key (SOPS) |
| `tailscale_accept_routes` | accept the advertised lab routes |
