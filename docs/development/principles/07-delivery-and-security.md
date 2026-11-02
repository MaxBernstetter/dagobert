# Delivery and security

How changes get from a commit to a running stack, and the security rules every piece follows. Local stack topology is in [local-stack](../../architecture/local-stack.md).

## Build and run

- **One way to run tasks:** every task runs through `task` (go-task) inside `devbox shell`. Taskfile targets glue; logic lives in Go or scripts under `scripts/`.
- **Images:** each job has a Dockerfile under `docker/<job>/` and is built with `task build:jobs`. The orchestrator starts them; they are not part of `compose.yaml`.
- **Deploy:** there is no deployment target yet. Local runs are `task up`. Infra for Floci (local) and Supabase (prod) is Terraform in `infra/terraform`, applied with `task infra:local` or `task infra:prod`.
- **CI:** no workflow runs the test tasks yet (`.github/workflows/` contains the protobuf check only). Until one does, run `task test` and `npm run build` locally and report the result in the pull request.

## Security

- No secrets in the repository. Use `.env` (gitignored) for local config. Dagobert needs no exchange credentials: it only reads public market data and simulates fills.
- The orchestrator mounts the Docker socket. Keep it on the internal Compose network and off any public port.

## Checks when planning a piece

- Does it need a new secret? It goes into the gitignored `.env` or the Terraform/SSM path, never into the repo.
- Does it expose anything new on a port? Check `compose.yaml` and the published ports.
- Can every new step run locally with one `task` command? Is any logic in a Taskfile that belongs in a script?
