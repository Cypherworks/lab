# claude

The AI-assisted development workbench. It installs the CI toolchain — Go, Node, Terraform, Ansible, ansible-lint, yamllint, checkov, pre-commit, shellcheck, actionlint, gh — plus Claude Code and tmux on a Debian instance, sets up a dedicated `claude` workbench user, and wires that user to commit and open PRs on GitHub under the estate's own bot identity. Code, tests, and linting run on lab infrastructure and match CI.

Part of the [`lab`](https://github.com/Cypherworks/lab) mechanism library: a generic, parameterised role. It installs generic mechanism, but does take site secrets — the GitHub App id/key, the OAuth token, the reviewer login — from your inventory and SOPS. It holds no estate credentials (only a GitHub App key), and container image builds are delegated to CI. Human login (Authentik LDAP identity + OpenBao SSH CA cert) is applied separately by the `sssd` and `ssh_ca_trust` roles; the box is reached by SSH over the Headscale overlay, not `incus exec`.

Full design: `lab-deploy` [`docs/claude-workbench.md`](https://github.com/Cypherworks/lab-deploy/blob/main/docs/claude-workbench.md).

## Requirements

- Debian 13 / Ubuntu host (the cloud image ships only the SSH client, so the role installs `openssh-server`).
- Collection `community.general` (npm module); `ansible.builtin` otherwise. Privilege escalation (`become`) to root.
- Outbound HTTPS to go.dev, releases.hashicorp.com, github.com, cli.github.com, and npm.
- An installed GitHub App and its private key, delivered via SOPS, for the bot identity.

## Usage

Become the workbench user (`sudo -iu claude`), then start or re-attach a persistent session with `claude-session` (tmux attach-or-create) so it survives an SSH disconnect. Sign-in is headless — `CLAUDE_CODE_OAUTH_TOKEN` is sourced from `~/.workbench-env`, so no interactive `/login` is needed. Land work with `cw-claude-push` and `cw-claude-pr`, not `git push`.

## Role variables

| Variable | Default | Description |
|----------|---------|-------------|
| `claude_go_version` | `1.25.12` | Go toolchain version (upstream tarball). Keep level with CI. |
| `claude_terraform_version` | `1.11.4` | Terraform version (HashiCorp release zip). Keep level with CI. |
| `claude_actionlint_version` | `1.7.7` | actionlint release version. Keep level with CI. |
| `claude_apt_packages` | see defaults | Distro packages, including `openssh-server`, git, tmux, jq, build-essential, python3/pipx, node/npm, ansible, ansible-lint, yamllint, shellcheck. |
| `claude_pipx_packages` | `[checkov, pre-commit]` | Tools not packaged for Debian; pipx installs them isolated into `/usr/local/bin`. |
| `claude_code_package` | `@anthropic-ai/claude-code` | Claude Code npm package (global install). |
| `claude_user` | `claude` | The workbench user that owns the repos, secrets, and Claude Code state. |
| `claude_home` | `/home/claude` | Workbench user home (`0700`). |
| `claude_operator_group` | `ssh-users` | LDAP group granted passwordless `sudo -iu claude`. |
| `claude_git_user_name` | `Claude Workbench` | Local git `user.name` for the workbench. |
| `claude_git_user_email` | `claude@cypherworks.co.uk` | Local git `user.email`. |
| `claude_repos` | lab, lab-deploy | Private repos cloned into `~/git/cw`, pulled with the App installation token. |
| `claude_oauth_token` | `""` | **Secret (SOPS).** `CLAUDE_CODE_OAUTH_TOKEN` for headless Claude Code sign-in. Empty skips credential wiring. |
| `claude_github_app_id` | `""` | The cw-claude GitHub App id — the box's whole GitHub identity. |
| `claude_github_app_private_key` | `""` | **Secret (SOPS).** The App private key (PEM), delivered to `claude_github_app_key_path`. |
| `claude_github_app_key_path` | `~/.config/cw-claude/app.pem` | Where the App key is written (`0600`), read by `cw-claude-token`. |
| `claude_github_reviewer` | `""` | PR reviewer + assignee login for `cw-claude-pr` (empty = don't set a reviewer). |
| `claude_config_repo` | `""` | A private repo cloned as `~/.claude` (operator rules, hooks, settings). Empty skips the config sync. |

`claude_github_app_id`, `claude_github_app_private_key`, and `claude_oauth_token` are secrets from SOPS. Pin the three version variables to a current release before apply and keep them level with the CI images. Leaving the secrets empty installs the toolchain but skips the credential wiring.

## Dependencies

None as Ansible role deps. The identity/login stack (`sssd`, `ssh_ca_trust`) is applied by the deployment's node playbook, not from here.

## What it does

Toolchain (as root): installs `claude_apt_packages` and enables `sshd`; adds the GitHub CLI apt repo and installs `gh`; installs Go, Terraform, and actionlint from upstream release archives (idempotent via `creates`) and pipx-installs `checkov` and `pre-commit`; installs Claude Code from npm, a base `/etc/tmux.conf`, and the `claude-session` helper; and installs the cw-claude helper scripts (below) into `/usr/local/bin`.

Workbench user (`user.yml`): creates the `claude` user, a sudoers drop-in granting `%{{ claude_operator_group }}` passwordless `sudo -iu claude`, and a `claude` launch alias; delivers the GitHub App key to `claude_github_app_key_path` (`0600`, `no_log`); renders `~/.workbench-env` (`0600`) exporting `CW_CLAUDE_APP_ID`, `CW_CLAUDE_APP_KEY`, `CW_CLAUDE_REVIEWER`, `CLAUDE_CODE_OAUTH_TOKEN`, and `GH_TOKEN="$(cw-claude-token)"`; sets the git identity and the `!cw-claude-credential` credential helper; clones `claude_repos` into `~/git/cw` and runs `pre-commit install` in each; and seeds `~/.claude.json` so headless Claude Code skips onboarding.

Operator rules (`config.yml`, only when `claude_config_repo` is set): clones it as `~/.claude` (user-scope `CLAUDE.md`, `rules/`, hooks, settings).

## The cw-claude GitHub App tooling

The workbench commits as a bot, not a person. A GitHub App can't register an SSH signing key, so a plain `git push` from it is never Verified; instead the helpers mint short-lived tokens and land commits through the GitHub API:

- **`cw-claude-token`** — signs a short-lived RS256 JWT with the App key, exchanges it for an installation access token (~1h), and caches it (`~/.cache/cw-claude/token.json`), re-minting only near expiry.
- **`cw-claude-credential`** — a git credential helper serving that token for `github.com` only.
- **`gh`** — a wrapper on the PATH ahead of the apt `gh` that exports a fresh `GH_TOKEN` from `cw-claude-token` before delegating to the real gh, so a long session never carries an expired token.
- **`cw-claude-push`** — recreates the local branch as one Verified commit authored by `cw-claude[bot]` via the `createCommitOnBranch` GraphQL mutation, then resyncs the local branch. Use it instead of `git push`.
- **`cw-claude-pr`** — opens a draft PR with validated `--type`/`--area`/`--priority` labels, optional `--closes`/`--refs` links, and the reviewer/assignee from `CW_CLAUDE_REVIEWER`.

## Notes

- The workbench holds no estate credentials — only the GitHub App key, which touches no lab resource. Container image builds are delegated to CI, not run here.
- `GH_TOKEN` in the environment is a `ghs_` installation token, not a PAT; `gh api /user` returns 403 for it (an App token is not a user), which is normal.
