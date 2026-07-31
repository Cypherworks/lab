# github_runner

Deploys a self-hosted GitHub Actions runner as a container on a Docker host.
The runner registers **ephemeral** — it de-registers after every job, so no
state carries between runs — against a single repository or a whole
organisation, using a fine-grained PAT.

It runs jobs **natively**: no Docker socket is mounted, so workflows must use
downloaded tools (pip, release binaries, `setup-*` actions) rather than
container actions. This keeps the runner unprivileged.

## What it does

1. Creates the data + work directories on the host.
2. Renders `compose.yaml` from the pinned image and registration settings.
3. Brings the stack up with `docker_compose_v2` (idempotent).

## Required variables

| Variable | Purpose |
|----------|---------|
| `github_runner_image` | Pinned `myoung34/github-runner` tag. |
| `github_runner_access_token` | Fine-grained PAT, from SOPS (repo admin for repo scope; org self-hosted-runner admin for org scope). |
| `github_runner_repo_url` | Repo URL to register against (repo scope only). |

## Key defaults

| Variable | Default | Purpose |
|----------|---------|---------|
| `github_runner_name` | `<host>-ci` | Runner + container name. |
| `github_runner_labels` | `self-hosted,nas` | `runs-on` targeting labels. |
| `github_runner_scope` | `repo` | Register scope: `repo`, `org`, or `ent`. |
| `github_runner_org` | `""` | Org name when scope is `org`. |
| `github_runner_data_dir` | `/opt/github-runner` | Compose + work root. |
| `github_runner_ephemeral` | `true` | De-register after each job. |
| `github_runner_pull` | `always` | Re-pull the image each run so a deprecated runner version self-heals. |
| `github_runner_security_opt` | `[]` | Extra container `security_opt` entries; empty keeps the default seccomp profile. |

`github_runner_docker_cli` has no default: it is unset in the role and falls back to
`default(omit)` at the compose step, so `docker` is found on PATH. Set it to the Synology
docker path only when `docker` is not on the ansible user's PATH.

## Prerequisites

The PAT is fine-grained. For **repo** scope, scope it to the one repository with
**Administration: Read and write**. For **org** scope, use an **organization**
PAT with self-hosted-runner management. Either is used to fetch a runner
registration token. Store it in SOPS and pass it as `github_runner_access_token`.
