# Delivery and security

How changes get from a commit to a running stack, and the security rules every piece follows. Local stack topology is in [local-stack](../../architecture/local-stack.md).

## Build and run

- **One way to run tasks:** every task runs through `task` (go-task) inside `devbox shell`. Taskfile targets glue; logic lives in Go or scripts under `scripts/`.
- **Images:** each job has a Dockerfile under `docker/<job>/` and is built with `task build:jobs`. Step Functions and ECS run them; they are not compose services.
- **Deploy:** there is no deployment target yet. Local runs are `task up`. Infra is one Terraform root in `infra/terraform` (VPC, Aurora, S3, SSM, IAM). `environment=local` targets Floci with fake credentials (`task infra:local`); `environment=prod` targets real AWS and is refused unless `CI=true`, AWS credentials are present and `allow_cloud_deploy` is set (`task infra:prod`, run by CI only).
- **CI:** `.github/workflows/tests.yml` runs `go build`, `go vet`, all unit tests and all system tests (with Postgres and the C++ strategy executables), the `dagobert-backtest` image build, and the frontend typecheck and build. It runs on every pull request, on every push to main, and nightly. With `CI` set, missing prerequisites fail the tests instead of skipping them. A red run on main opens or updates an issue labeled `ci-broken` with the commit. Mark the `tests` jobs as required checks in the branch protection rules.

## Security

- No secrets in the repository. Use `.env` (gitignored) for local config. Dagobert needs no exchange credentials: it only reads public market data and simulates fills.
- Floci mounts the Docker socket (it starts the Aurora, ECS and Lambda containers). Its ports are published on `127.0.0.1` only.

## Checks when planning a piece

- Does it need a new secret? It goes into the gitignored `.env` or the Terraform/SSM path, never into the repo.
- Does it expose anything new on a port? Check `compose.yaml` and the published ports.
- Can every new step run locally with one `task` command? Is any logic in a Taskfile that belongs in a script?
