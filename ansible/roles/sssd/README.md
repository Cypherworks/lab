# sssd

LDAP **identity** (NSS) against the Authentik LDAP outpost. Gives a host LDAP-backed
users and groups so `id`, `getent passwd`, and `getent group` resolve Authentik
accounts.

## Scope — identity only

This role deliberately does **not** touch PAM or sshd. It cannot affect login or
sudo, so it's safe to apply to a host without risking connectivity. Verify with:

```
id <a-directory-user>
getent group ssh-users
```

The auth side (PAM sudo password, `ssh-users` access filter, `pam_mkhomedir`) and the
OpenBao SSH CA trust (`TrustedUserCAKeys`) are a separate, carefully-rolled-out step.

## Schema notes

The Authentik LDAP tree is a hybrid (AD-ish `objectClass=user`/`group` + `posixAccount`
+ rfc2307bis `member` DNs). Two gotchas the config handles:

- The login name is `cn`; `uid` is a stable hash, so `ldap_user_name = cn`.
- No `loginShell` is served, so `default_shell` is set (else logins get no shell).

## Data

Site values (URIs, base DN, bind DN, CA PEM) come from the deploy; the bind app
password resolves from SOPS (`ldap_search_password`, shared with the Authentik ldap
blueprint).
