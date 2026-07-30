# claude

The AI-assisted development workbench. Installs the CI toolchain — Go, Node, OpenTofu,
ansible, ansible-lint, yamllint, checkov, shellcheck, actionlint, gh — plus Claude Code
and tmux on a Debian instance, so code, tests, and linting run on lab infrastructure and
match CI.

Mechanism only: the role installs the toolchain and nothing site-specific. It holds no
estate credentials, and container image builds are delegated to CI. Human login
(Authentik LDAP identity + OpenBao SSH CA cert) is applied separately by the `sssd` and
`ssh_ca_trust` roles; the box is reached by SSH over the Headscale overlay, not
`incus exec`.

Full design: `lab-deploy` `docs/claude-workbench.md`.

## Usage

Sign in to Claude Code interactively — run `claude` and `/login` (OAuth); the role
stores no key. Start or re-attach a persistent session with `claude-session`
(tmux attach-or-create), so the session survives an SSH disconnect.

## Variables

The pinned binary versions (`claude_go_version`, `claude_tofu_version`,
`claude_actionlint_version`) must be confirmed against a current release before apply
and kept level with the CI images.
