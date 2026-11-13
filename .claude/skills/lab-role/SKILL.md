---
name: lab-role
description: Write or change a role, Terraform module or script in the public Cypherworks/lab repo. Use when adding or editing anything under ansible/roles, terraform/modules or scripts in lab, when a lab change must be wired into lab-deploy, or before committing to lab, which is public and must carry no site data.
---

# Working in lab (public mechanism repo)

`lab` is public. It holds generic Ansible roles, Terraform modules and
scripts. All site data (domains, IPs, hostnames, emails, secrets) lives in
the private `lab-deploy` repo, which consumes these roles through
`roles_path` from a side-by-side clone (see its `lab-ops` skill). Follow the
generic `ansible-role` skill for role design; this skill adds what's
specific to lab.

## No site data, ever

- Defaults, comments, READMEs and examples use placeholders only:
  `example.com`, `admin@example.com`, `10.0.x` for lab addresses,
  `192.0.2.x` (RFC 5737) for the house network, `node-N` for hosts.
- A role that needs a site value defaults to a placeholder or empty plus an
  `assert`; the real value goes in lab-deploy's group_vars or host_vars.
- Before every commit, run the site-data leak check. Its patterns are real
  site values, so they live only in the private repo: read the "Public-repo
  leak check" section of `../lab-deploy/.claude/skills/lab-ops/SKILL.md`
  and run it against this diff. Never copy those patterns into this repo.
  Also check the diff for real inventory hostnames and personal names.
- SOPS hides secrets, not topology. A plaintext IP plan or host list is
  still a leak.

## Conventions here

- Roles share a lab vocabulary (`lab_*`, `ops_*` variables supplied by the
  deploy), so `var-naming[no-role-prefix]` is skipped in `.ansible-lint`.
  Role-private variables still take the role prefix.
- `defaults/` in every role; `vars/` only where a role truly needs internal
  constants. README sections: Requirements, Role variables (table),
  Dependencies, What it does, Example, Notes.
- Tests are playbooks in `ansible/roles/<role>/tests/`, mostly
  Docker-based, with a header comment saying what they prove and how to
  run them from `ansible/`, e.g.
  `ansible-playbook -i localhost, roles/caddy/tests/secret_env.yml`.
  Only `github_runner` has Molecule. Write the test before the role change
  (`tdd-guide`).
- Terraform modules need Terraform `>= 1.10`; UniFi modules pin
  `filipowm/unifi` `1.0.0`.
- Scripts in `scripts/` follow `operator-script`; `shellcheck` must pass.

## Validate before committing

Run what CI runs (`.github/workflows/pr.yml`, GitHub-hosted runners):

```bash
pre-commit run --all-files
ansible-lint
yamllint .
shellcheck scripts/*.sh
```

Plus the role's own test playbooks. `security.yml` runs actionlint,
zizmor, Checkov, Trivy and an SBOM job, and every scanner hard-fails;
handle findings per `scanner-findings` (an accepted Checkov finding is an
inline `#checkov:skip=<id>:<reason>`). Workflow actions are pinned to full
commit SHAs.

## Wiring a role into lab-deploy

There's no `requirements.yml`; roles resolve through lab-deploy's
`roles_path`. The lab-deploy side needs a playbook in
`ansible/playbooks/`, an entry in `site.yml` in dependency order, and the
site values in its inventory. Do the lab PR and the lab-deploy PR as a
pair, and land lab first.

## PRs

Public repo CI stays on GitHub-hosted runners: never add `self-hosted`
jobs here (a fork PR could run code on the lab). Issues are tracked in
`Cypherworks/lab-deploy`; a lab PR closes them with
`Closes Cypherworks/lab-deploy#N`. Labels mirror lab-deploy's, except
`area:network` doesn't exist here (use `area:devsecops`). Commit and hand
off per `ship-change`.
