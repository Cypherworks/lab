# github_runner

Deploys a self-hosted GitHub Actions runner as a container on a Docker host
(a Synology NAS in this lab). The runner registers **ephemeral** — it
de-registers after every job, so no state carries between runs — against a
single repository using a fine-grained PAT.

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
| `github_runner_access_token` | Fine-grained PAT (repo administration r/w), from SOPS. |
| `github_runner_repo_url` | Full repo URL to register against. |

## Key defaults

| Variable | Default | Purpose |
|----------|---------|---------|
| `github_runner_name` | `<host>-ci` | Runner + container name. |
| `github_runner_labels` | `self-hosted,nas` | `runs-on` targeting labels. |
| `github_runner_data_dir` | `/opt/github-runner` | Compose + work root. |
| `github_runner_ephemeral` | `true` | De-register after each job. |
| `github_runner_docker_cli` | `omit` | Synology docker path if not on PATH. |

## Prerequisites

The PAT is fine-grained, scoped to the one repository, with **Administration:
Read and write** (needed to fetch a runner registration token). Store it in
SOPS and pass it as `github_runner_access_token`.
