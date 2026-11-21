# github_runner

Deploys a self-hosted GitHub Actions runner as a container on a Docker host.
The runner registers **ephemeral** — it de-registers after every job, so no
state carries between runs — against a single repository or a whole
organisation, using a fine-grained PAT.

It runs jobs **natively**: no Docker socket is mounted, so workflows must use
downloaded tools (pip, release binaries, `setup-*` actions) rather than
container actions. This keeps the runner unprivileged.

The one exception is `github_runner_mount_docker_socket` (see below).

## What it does

1. Asserts the image pin, the access token, and the registration target (repo URL
   for `repo` scope, org name for `org` scope) are set, failing early otherwise.
2. With `github_runner_group_repository_ids` set, makes sure the org runner
   group exists and only those repositories can use it (see below).
3. Creates the data + work directories on the host. The work dir is mounted
   into the runner at its host path, so a job's `docker run -v "$PWD:..."`
   (resolved by the host's daemon when the socket is mounted) sees the checkout.
4. Renders the job-start cleanup hook when it is on.
5. Renders `compose.yaml` (mode `0600`, it holds the PAT) from the image and
   registration settings.
6. Brings the stack up with `docker_compose_v2` (idempotent).

Docker Engine and Compose come from the `docker` role (`meta/main.yml` dependency).

## Required variables

| Variable | Purpose |
|----------|---------|
| `github_runner_image` | Pinned `myoung34/github-runner` tag. |
| `github_runner_access_token` | Fine-grained PAT, from SOPS (repo admin for repo scope; org self-hosted-runner admin for org scope). |
| `github_runner_repo_url` | Repo URL to register against (repo scope only). |
| `github_runner_org` | Org name to register against (org scope only). |

## Key defaults

| Variable | Default | Purpose |
|----------|---------|---------|
| `github_runner_name` | `<host>-ci` | Runner + container name. |
| `github_runner_labels` | `self-hosted,nas` | `runs-on` targeting labels. |
| `github_runner_scope` | `repo` | Register scope: `repo`, `org`, or `ent`. |
| `github_runner_group` | `""` | Runner group to join (`RUNNER_GROUP`). Empty leaves it unset, so the runner joins the Default group. |
| `github_runner_data_dir` | `/opt/github-runner` | Compose + work root. |
| `github_runner_ephemeral` | `true` | De-register after each job. |
| `github_runner_pull` | `always` | Re-pull the image each run so a deprecated runner version self-heals. |
| `github_runner_security_opt` | `[]` | Extra container `security_opt` entries; empty keeps the default seccomp profile. |
| `github_runner_mount_docker_socket` | `false` | Bind-mount the host's `/var/run/docker.sock` into the runner so jobs can drive Docker (Molecule's docker driver). Read the section below first. |
| `github_runner_job_started_cleanup` | `false` | Before each job, remove what earlier jobs left on the host's daemon. Needs the socket mount. See below. |
| `github_runner_group_repository_ids` | `[]` | Numeric IDs of the only repositories allowed to use `github_runner_group`. The role creates or repairs the group to match. Org scope only. See below. |
| `github_runner_api_url` | `https://api.github.com` | GitHub REST API base for the runner group calls. |

`github_runner_docker_cli` has no default: it is unset in the role and falls back to
`default(omit)` at the compose step, so `docker` is found on PATH. Set it to the Synology
docker path only when `docker` is not on the ansible user's PATH.

## Mounting the Docker socket

Off by default, and it should stay off everywhere except a runner built for it.
Access to the Docker socket is root on the host: any job can
`docker run --privileged -v /:/host` and own the machine the runner sits on,
read every container's environment with `docker inspect` (including this
runner's own `ACCESS_TOKEN`), and reach anything the host can reach on the
network. The runner container is no longer a boundary.

That trade is only acceptable when the host has nothing worth taking. It must
be single-purpose, running this runner and nothing else, so owning it yields
no other service, data or co-tenant. It must hold no credentials beyond the
runner's own registration PAT: no lab secrets, SSH keys, cloud creds or
Incus/OpenBao access. (Job code can read that PAT with or without the socket,
so scope it to runner registration and nothing more.) It must be a VM on its
own kernel, not an LXC container sharing a node's kernel, with the runner
registered `ephemeral` and the VM rebuildable from code, so a compromised host
is rebuilt rather than cleaned. And it should sit on a segment with internet
egress only, firewalled off from the lab and management networks.

If any of that doesn't hold, don't enable it. Jobs that only need to build or
lint run fine natively on the default runners.

## Clearing the daemon before each job

A runner with the socket shares one daemon across every job it runs, so a
container a cancelled job left behind (or a fixed `--name` it used) can break
the next job. `github_runner_job_started_cleanup: true` renders
`<data_dir>/hooks/job-started.sh`, mounts it read-only and points
`ACTIONS_RUNNER_HOOK_JOB_STARTED` at it, so the runner runs it before every
job. It removes every container except the runner's own, then unused
networks and all unused volumes. Images and build cache stay, so builds stay
warm; prune those on a schedule with the `docker` role's
`docker_prune_on_calendar`. A non-zero exit fails the job before its first
step. It removes every other container on the daemon, so use it only on a
host that runs nothing but this runner.

## Limiting the runner group to selected repositories

With `github_runner_scope: org`, `github_runner_group` set and
`github_runner_group_repository_ids` listing repository IDs, the role calls
GitHub's REST API once from the controller, with the runner's own token.
It creates the group if it's missing, with repository access set to the
selected repositories and public repositories off. If the group drifted
(visible to all, public repositories allowed, or a different repository
list), it puts it back. A second run only reads. The token needs the
organization permission Self-hosted runners: Read and write, which org
registration needs anyway. The calls are `no_log` because they carry the
token; a failed call reports GitHub's status and message instead.

## Prerequisites

The PAT is fine-grained. For **repo** scope, scope it to the one repository with
**Administration: Read and write**. For **org** scope, use an **organization**
PAT with self-hosted-runner management. Either is used to fetch a runner
registration token. Store it in SOPS and pass it as `github_runner_access_token`.

## Testing

`molecule/default` converges the role (with its `docker` dependency) in two
privileged Debian 13 containers: `runner-default` on the role defaults and
`runner-socket` with the socket mount and `github_runner_group` set. The verifier
checks the socket mount and `RUNNER_GROUP` in both the rendered compose file and
the running container.

`tests/docker_runner.yml` converges the role in a privileged systemd container
with its own dockerd, on Ubuntu 24.04 or (with `-e _image=debian:13`) Debian
13, as a socket-mounted runner with the cleanup hook and the docker role's
prune timer on. It checks the work dir path, that the hook clears leftovers
but keeps the runner and its image, the prune, idempotence and the refusals.
`tests/runner_group.yml` runs the runner group tasks against
`tests/github_api_stub.py`, a local stub of GitHub's runner-group endpoints.
Each file's header says how to run it.
