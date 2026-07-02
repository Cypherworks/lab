# sssd

Configures SSSD as an LDAP client against an Authentik LDAP outpost: identity/NSS by default, with optional PAM login and sudo.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. Supply site data (IPs, secrets, hostnames) from your inventory and SOPS, not from the role.

## Requirements

- Debian/Ubuntu host with the SSSD and LDAP client packages available.
- Collections: none beyond `ansible.builtin` (`apt`, `copy`, `template`, `lineinfile`, `systemd_service`, `command`).
- LDAPS reachability to the Authentik outpost endpoints and the CA the outpost certificate chains to.
- Privilege escalation (`become`) to root.
- For PAM auth: `pam-auth-update`, `libpam-sss`, `libpam-modules`, and `/usr/sbin/visudo`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `sssd_packages` | `[sssd, sssd-ldap, sssd-tools, libnss-sss, ldap-utils]` | Client packages installed. |
| `sssd_domain_name` | `lab` | Local SSSD domain label (arbitrary name). |
| `sssd_ldap_uris` | `[]` | LDAPS outpost endpoints; SSSD fails over between them. **Required from inventory.** |
| `sssd_ldap_base_dn` | `""` | Directory base DN, e.g. `dc=ldap,dc=example,dc=com`. **Required from inventory.** |
| `sssd_ldap_user_search_base` | `"ou=users,{{ sssd_ldap_base_dn }}"` | User search base. |
| `sssd_ldap_group_search_base` | `"ou=groups,{{ sssd_ldap_base_dn }}"` | Group search base. |
| `sssd_ldap_bind_dn` | `""` | Search/bind service-account DN (the `ldap-search` account). **Required from inventory.** |
| `ldap_search_password` | `""` | Bind account app password. **Secret — from SOPS** (`ldap_search_password`, shared with the Authentik blueprint). |
| `sssd_ldap_ca_cert` | `""` | PEM of the CA the outpost LDAPS cert chains to (public, not secret). **Required from inventory.** |
| `sssd_ldap_ca_cert_path` | `/etc/sssd/openbao-ca.pem` | Path the CA PEM is written to. |
| `sssd_default_shell` | `/bin/bash` | Default shell; required because the Authentik schema serves no `loginShell`. |
| `sssd_fallback_homedir` | `/home/%u` | Fallback home directory template. |
| `sssd_nsswitch` | `[{db: passwd, value: "files systemd sss"}, {db: group, value: "files systemd sss"}]` | NSS databases pointed at SSSD (whole-line, idempotent). |
| `sssd_enumerate` | `false` | Look users up on demand rather than pulling the whole directory. |
| `sssd_auto_private_groups` | `hybrid` | Synthesize a user-private primary group when `uid == gid` and no real group owns that gid. |
| `sssd_enable_pam_auth` | `false` | Opt-in switch for PAM login/sudo. Off means identity-only (NSS). |
| `sssd_ssh_access_group` | `ssh-users` | LDAP group whose members may log in (PAM access filter on `memberOf`). |
| `sssd_sudo_group` | `ssh-sudoers` | LDAP group granted password-authenticated sudo; empty installs no sudo rule. |
| `sssd_pam_profiles` | `[sss, mkhomedir]` | PAM profiles enabled via `pam-auth-update` when auth is on. |
| `sssd_local_exempt_users` | `[root, ansible]` | Accounts SSSD must never resolve from LDAP; kept purely local. |

Required site data with no usable default: `sssd_ldap_uris`, `sssd_ldap_base_dn`, `sssd_ldap_bind_dn`, `sssd_ldap_ca_cert`. `ldap_search_password` is a secret and must come from SOPS.

## Dependencies

None.

## What it does

Identity path (always):

1. Installs `sssd_packages` via apt.
2. Writes `sssd_ldap_ca_cert` to `sssd_ldap_ca_cert_path` (root:root, mode `0644`).
3. Templates `sssd.conf.j2` to `/etc/sssd/sssd.conf` at mode `0600` (mandatory — SSSD refuses a looser config, and the file holds the bind password). Uses `ldap_schema = rfc2307bis` with `ldap_user_name = cn`, `auto_private_groups = hybrid`, `ldap_tls_reqcert = demand` against the CA.
4. Points `passwd`/`group` in `/etc/nsswitch.conf` at `sss` via whole-line `lineinfile`. `shadow` is deliberately left alone (auth territory).
5. Enables and starts the `sssd` service.

Config/CA changes notify the `Restart sssd` handler, which restarts (not reloads) to clear the aggressive cache so stale identity data does not linger.

PAM path (only when `sssd_enable_pam_auth | bool`):

6. Ensures `libpam-sss` and `libpam-modules` are installed.
7. Checks whether `pam_mkhomedir.so` is already active in `/etc/pam.d/common-session`, and only then runs `pam-auth-update --enable <profiles>` (guarded for idempotency).
8. Writes `/etc/sudoers.d/ssh-sudoers` granting `%<sssd_sudo_group> ALL=(ALL:ALL) ALL`, validated with `visudo -cf` before install, and only when `sssd_sudo_group` is non-empty. The template also adds `auth_provider`/`access_provider`/`ldap_access_filter` to `sssd.conf` and the `pam` service.

## Example

Identity only (default, safe on any host):

```yaml
- hosts: lab_hosts
  roles:
    - role: sssd
      vars:
        sssd_ldap_uris: [ldaps://ldap-a:636, ldaps://ldap-b:636]
        sssd_ldap_base_dn: dc=ldap,dc=example,dc=com
        sssd_ldap_bind_dn: cn=ldap-search,ou=users,dc=ldap,dc=example,dc=com
        ldap_search_password: "{{ vault_ldap_search_password }}"
        sssd_ldap_ca_cert: "{{ openbao_ca_pem }}"
```

Enabling login/sudo (one host at a time, with console access):

```yaml
      vars:
        sssd_enable_pam_auth: true
```

## Notes

By default this role is identity-only: `id_provider = ldap` gives the host LDAP-backed users and groups (`getent`, `id`) but touches neither PAM nor sshd, so it cannot affect login or sudo. Verify with `id <user>` before considering auth.

`sssd_enable_pam_auth` is connectivity-sensitive. Enable it only with console or break-glass access to the host, and roll it out one host at a time: it makes login and sudo depend on reachability to the LDAP outpost. Local `root` and `ansible` remain in `filter_users`/`filter_groups`, so they stay purely local and pam_unix continues to admit them; key-based SSH never hits the PAM auth stack.

The Authentik schema is hybrid: the login name is `cn` (the `uid` attribute is a hash, so an rfc2307 default would surface usernames as hashes), and no `loginShell` is served, which is why `sssd_default_shell` is required. `auto_private_groups = hybrid` resolves Authentik's nameless private primary groups without overriding users that have a genuine primary group.

On CIS-hardened hosts, verify the faillock/pwquality PAM config still composes after `pam-auth-update`; check at the rig with a held root session.
