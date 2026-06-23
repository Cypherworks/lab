# Ansible roles

Reusable, generic Ansible roles. Behaviour is parameterised through defaults and
variables; site-specific data (hosts, users, secrets) is supplied by the
consuming deployment via group_vars/host_vars/vault, never baked into a role.
