# automation

Go code for the AWS Lambdas that hold the lab automation's root of trust: the broker (issues short-lived apply credentials and down-scoped GitHub App tokens to verified workflow runs), the registrar (mints single-job runner configs for enrolled hosts) and the manifest function (creates the GitHub App and stores its key in SSM). Generic mechanism only; the deploy repo supplies account, org, repository and actor IDs.

It also holds `runner-supervisor`, a daemon that runs on a lab host rather than in AWS: it resets a Docker runner VM to its golden Incus snapshot after every job and registers the clean VM with a single-job JIT config, so the registration credential never enters the VM. Design: lab-deploy `docs/docker-runner-reset.md`.

Design: Cypherworks/lab-deploy#245.

## Layout

- `internal/oidc` verifies GitHub Actions OIDC ID tokens (RS256 against GitHub's JWKS, issuer, audience, exp/nbf/iat with 30s leeway) and returns the claims. It doesn't authorise them.
- `internal/runners` mints JIT runner configs and reads, lists and deletes an org's self-hosted runners with a bearer token. Its errors never carry a response body, since a mint's body is a credential.
- `internal/incus` drives instances in one Incus project through the `incus` CLI, so a client certificate restricted to that project is all it needs.
- `internal/supervisor` is the per-VM loop: restore, boot, mint, hand the config in on stdin, then watch GitHub and Incus (never the VM) for the job's end. It writes node_exporter textfile metrics.
- `cmd/runner-supervisor` runs one VM's loop. SIGTERM drains it; exit code 3 means the VM's reset kept failing and it's left stopped.

More packages and the `cmd/<function>` entry points land in later PRs.

## Build and test

Go 1.25, standard library only so far.

```bash
cd automation
go vet ./...
go test -race ./...
```

Tests run real logic: tokens are signed with RSA keys generated per test, and an `httptest` server stands in for GitHub's JWKS endpoint. The supervisor's tests drive the loop with a fake clock against in-memory GitHub and Incus stand-ins, and the `incus` wrapper's tests run a stand-in `incus` script. `httptest` binds a local port, so the tests fail inside a sandbox that forbids local binding.

`runner-supervisor` ships as a static binary: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o runner-supervisor ./cmd/runner-supervisor`.

Lambdas run on the `provided.al2023` runtime as a single static `bootstrap` binary (`CGO_ENABLED=0`, `-trimpath`, `-tags lambda.norpc`).
