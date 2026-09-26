# automation

Go code for the AWS Lambdas that hold the lab automation's root of trust: the broker (issues short-lived apply credentials and down-scoped GitHub App tokens to verified workflow runs), the registrar (mints single-job runner configs for enrolled hosts) and the manifest function (creates the GitHub App and stores its key in SSM). Generic mechanism only; the deploy repo supplies account, org, repository and actor IDs.

Design: Cypherworks/lab-deploy#245.

## Layout

- `internal/oidc` verifies GitHub Actions OIDC ID tokens (RS256 against GitHub's JWKS, issuer, audience, exp/nbf/iat with 30s leeway) and returns the claims. It doesn't authorise them.

More packages and the `cmd/<function>` entry points land in later PRs.

## Build and test

Go 1.25, standard library only so far.

```bash
cd automation
go vet ./...
go test -race ./...
```

Tests run real logic: tokens are signed with RSA keys generated per test, and an `httptest` server stands in for GitHub's JWKS endpoint.

Lambdas run on the `provided.al2023` runtime as a single static `bootstrap` binary (`CGO_ENABLED=0`, `-trimpath`, `-tags lambda.norpc`).
