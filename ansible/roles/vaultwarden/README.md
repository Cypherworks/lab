# vaultwarden

Runs Vaultwarden (Bitwarden-compatible password manager) on one Incus instance via docker compose, SQLite-backed.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- `community.docker` collection.
- A container/host the `docker` role can install Docker Engine and Compose v2 onto.
- `vaultwarden_domain` set, and `vaultwarden_admin_token` supplied from SOPS if the admin panel is wanted.
- Root via `become`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `vaultwarden_compose_dir` | `/opt/vaultwarden` | Host directory for the compose file, env file, and the SQLite `/data` volume. |
| `vaultwarden_image` | `vaultwarden/server:1.36.0` | Pinned container image. Confirm the registry tag exists before apply. |
| `vaultwarden_http_port` | `8080` | Host port mapped to the container's HTTP (`:80`); Caddy reverse-proxies to this and carries the websocket upgrade for live sync. |
| `vaultwarden_domain` | `""` | Public URL Caddy serves it on. Required for WebAuthn/2FA, attachments, and links. Set by the deploy. |
| `vaultwarden_signups_allowed` | `false` | Whether public signups are open. Off by default. |
| `vaultwarden_admin_token` | `""` | Admin panel token, an argon2 PHC string from SOPS. Generate with `docker run --rm -it vaultwarden/server /vaultwarden hash`. Empty disables the admin panel entirely. |

## Dependencies

- `docker` role (meta dependency) — installs Docker Engine and Compose v2.

## What it does

Creates `vaultwarden_compose_dir`, renders the env file (config plus the admin token, mode `0600`) and the compose file, then brings the stack up with `docker_compose_v2`. Data lives in SQLite in the `/data` volume, so the service is self-contained with the fewest possible dependencies, as befits a root-of-trust service.

Changes to the env or compose file restart the stack.

## Example

```yaml
- hosts: vaultwarden_host
  roles:
    - role: vaultwarden
      vars:
        vaultwarden_domain: "https://vault.example.com"
        vaultwarden_admin_token: "{{ vault_vaultwarden_admin_token }}"
```

Open signups for a single registration, then set it back to `false`:

```yaml
        vaultwarden_signups_allowed: true
```

## Notes

Vaultwarden is deliberately not placed behind Authentik forward-auth. The vault is already zero-knowledge (the master password derives the encryption key; the server cannot decrypt the data), and fronting it with the IdP whose own password it stores would create a circular lock-out and break the native Bitwarden clients. It is protected instead by its master password plus in-app 2FA, and kept LAN/overlay-only via Caddy.

Signups off (`vaultwarden_signups_allowed: false`) is the secure default. Bootstrap the first account via an admin-panel invite, which works with signups disabled, or flip the flag true for a single registration and back.
