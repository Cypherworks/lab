# vcenter_oidc

Configures vCenter Server identity provider federation against an external OIDC
provider (Authentik) through the vSphere Automation API. This is the ESXi/vCenter
counterpart of the proxmox role's OIDC realm. The Authentik side (provider, app,
admin group) is the `authentik_app` role's `vcenter` blueprint.

## Status: unofficial

vCenter 8.0 officially federates only AD FS, Okta, Microsoft Entra ID, and
PingFederate — there is no generic-OIDC option in the UI. Authentik works through
the API (Broadcom staff and Authentik both document it), but it is not a supported
provider, so the exact `CreateSpec` and the tenant path can shift between releases.
Treat the spec below as a starting point and confirm it against the live API.

## Where it runs

On the **control host** (a `localhost` play), after `vcsa` has stood vCenter up.
It creates an API session (`POST /api/session`), lists providers to stay idempotent,
and `POST`s the provider spec when one named `vcenter_oidc_name` is absent.

## The spec is site data

The role does not encode the `CreateSpec` — it POSTs `vcenter_oidc_spec` verbatim, so
the (version-specific) shape lives in the deploy where it can be corrected without
touching the role. Set it in `build_hosts`/`vcenter` group_vars, with the client
secret pulled from SOPS. Example to confirm against the API:

```yaml
vcenter_oidc_name: authentik
vcenter_oidc_spec:
  config_tag: Oauth2
  is_default: true
  name: authentik
  domain_names: [cypherworks.co.uk]
  upn_claim: email
  groups_claim: groups
  oauth2:
    client_id: "{{ vcenter_oidc_client_id }}"
    client_secret: "{{ vcenter_oidc_client_secret }}"
    discovery_endpoint: "https://auth.cypherworks.co.uk/application/o/vcenter/.well-known/openid-configuration"
    claim_map: {}
```

## Verify on the box

- The `CreateSpec` field names and whether the sub-object is `oauth2` or `oidc`, and
  the exact `claim_map` structure for group mapping. Read a manually-configured
  provider back from `GET /api/vcenter/identity/providers/{provider}` to see the shape.
- The redirect URI to register in Authentik (`vcenter_oidc_redirect_uris`):
  `https://<vcenter-fqdn>/federation/t/CUSTOMER/auth/response/oauth2` — confirm the
  `CUSTOMER` tenant name for this vCenter.
- The provider-list field the idempotency check matches on (`name`).

## Key variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `vcenter_oidc_ip` | `""` | vCenter IP/host for the API session. |
| `vcenter_oidc_sso_password` | `""` | `administrator@vsphere.local` password (SOPS). |
| `vcenter_oidc_name` | `""` | Provider name; idempotency key. |
| `vcenter_oidc_spec` | `{}` | The `CreateSpec` body POSTed to the API. |
