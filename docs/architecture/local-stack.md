# Local stack

`task up` leaves two compose services running, Floci and the orchestrator. Floci starts the Aurora database, the S3 buckets and the web app's CloudFront distribution from the Terraform in `infra/terraform`. It does not run anything by itself: experiments and backfills start when you ask for them.

## What stays up

| Process | Role |
|---|---|
| Web app (S3 + CloudFront in Floci) | The production Vite SPA (`src/frontend`), built by `scripts/frontend-deploy.sh local` with the Experiment API on the same origin, uploaded to the private bucket `dagobert-local-frontend`. The CloudFront distribution (Terraform `frontend.tf`) serves the bucket and routes `/tradingcopilot.v1.*` to the Experiment API gateway. Served at the `frontend_domain` output through Floci's CloudFront (Host header). Floci does not run the SPA-fallback function, so deep links 404 locally (see Acceptance). For hot reload use `task frontend:dev`. |
| Aurora (in Floci) | Aurora PostgreSQL Serverless v2, created by Terraform (`task infra:local`, from `infra/terraform`) and served by Floci on port 7001 (TLS, `sslmode=require`). Compose services reach it as `floci:7001`, host tools as `127.0.0.1:7001`. Catalog of experiments and instances, and the connector state (`provider`, `provider_pair`). The orchestrator mounts the Experiment API and the connector routes when `DATABASE_URL` is set. Secrets are not in Postgres. |
| Persistence | Candles: Hive-style S3 keys `parquet/provider={id}/base={B}/quote={Q}/year={YYYY}/candles.parquet` (footer metadata) in the Floci bucket (`LAKE_BACKEND=s3`, the compose default; `fs` keeps them under `./data/parquet`). Ingest and the indicator job (`parquet/ti/`) upload and keep no local copy; readers download to a temp file. The ingest ledger (`ingest_stored`, `ingest_unavailable`) is in Aurora; `task data:migrate-ingest-ledger` imports an old `ingest.sqlite`. Experiment workspace data (`experiments/<experiment_id>/shards/...` result Parquet and `experiments/<experiment_id>/.cache/...` handoff files) uses the same `LAKE_BACKEND` and bucket: objects under the data-dir-relative key with `s3`, files under `./data/experiments/` with `fs` (`src/internal/workspace`). The `./data` volume holds `backtests.sqlite` (legacy single-backtest catalog) and is the key root and scratch location (`DATA_DIR` stays required). The jobs and the orchestrator mount this directory; the frontend does not. The experiment catalog and the connector state live in Postgres; `DATA_DIR/providers/secrets.json` holds the connector secrets. To move an existing `./data/parquet` lake into the bucket once, run `task lake:upload`. `task up` creates `data/parquet` on the host; the orchestrator ensures that layout before each job and runs job containers as `HOST_UID`/`HOST_GID` (from `task`) so bind mounts are not left root-owned. |
| Experiment API gateway (in Floci) | The Experiment API as a container-image Lambda (`<prefix>-experiment-api`) behind an API Gateway HTTP API, created by Terraform (`api_lambda.tf`). On AWS call the `api_gateway_url` output: `POST {url}/tradingcopilot.v1.ExperimentService/<Method>`; on Floci use `http://127.0.0.1:11566/_aws/execute-api/<api-id>/$default/...`. The CloudFront distribution in front of the web app routes to this gateway. Known gaps: no auth, about 6 MB payload limit, no RDS Proxy, Run/Cancel hooks write the database only, and SSM is not reachable from the VPC Lambda. |
| Orchestrator | Floci on host port `FLOCI_PORT` (default 11566), and a starter process that owns the Docker socket. Also serves the Experiment API and the connector routes (`/api/v1/providers`, Connectors page) on `ORCHESTRATOR_PORT` (default 11090). The connector routes need `DATABASE_URL`; the Binance base URL is the saved config, else `BINANCE_BASE_URL`, else the public default. |

The job images (`dagobert-ingest`, `dagobert-calculation-technical-indicator`, `dagobert-backtest`) are runners. They appear when a backfill or an experiment needs them and exit when done.

The candle lake is in Floci S3 (bucket `dagobert-local-lake`, created by Terraform). The SQLite files still persist on `./data`; experiment shards and the prepare cache are in the same bucket under `experiments/<id>/` when `LAKE_BACKEND=s3` (the compose default).

## What the orchestrator does

1. On startup it checks the data layout and serves the HTTP API on `ORCHESTRATOR_PORT`: backfill, provider connectors and (with `DATABASE_URL`) the Experiment API.
2. A backfill or an experiment tells it which image to run and which arguments to pass. It runs that image, waits until it exits and reads the code. Exit `0` and exit `2` are success; exit `1` fails.
3. Data jobs run one at a time. Experiment fan-out is described in [experiment-data-flow.md](experiment-data-flow.md).

Floci's Lambda cannot see the host Docker socket. The orchestrator is the process that runs the images, with the socket mounted.

## Commands

| Command | Effect |
|---|---|
| `task build` | Job images plus compose images (the orchestrator). |
| `task build:jobs` / `task build:stack` | Build only the job images or only the compose services. |
| `task up` | `task db:migrate` (which applies Terraform on Floci first, then uploads the web app), then `docker compose up`. |
| `task dev` | `task build:jobs`, `task build:stack`, `task db:migrate`, then `docker compose up`. `db:up` also creates the roles `anon` and `authenticated` the migrations revoke from. |
| `task frontend:dev` | Vite dev server with hot reload on the host (port 3000), proxying `/api` and `/tradingcopilot.v1.` to the orchestrator. |
| `task up:build` | `task build` then `task up` (job images + compose build). |
| `task down` | Stops the stack. Does not delete `./data`. |
| `task db:migrate:ecs` | Runs the same Atlas migrations as a one-off ECS Fargate task (`scripts/db-migrate.sh local`, image `docker/db-migrate`), the path prod uses from inside the VPC. Waits for the task and fails unless the container exits 0. Floci's ECS and exit-code reporting are not verified yet. |

Dockerfiles use BuildKit cache mounts (`npm ci`, `go mod download`, `go build`) so rebuilds reuse work when only application source changed. The SPA is not in a Docker image: `scripts/frontend-deploy.sh` builds it and uploads `dist/` to S3.

`compose.yaml` does not list the jobs; the orchestrator starts them. `task up`, `task up:build` and `task dev` apply Terraform (creates the VPC and the Aurora cluster, idempotent) and the Atlas migrations before starting compose. The Atlas CLI is not in the devbox shell.

## Acceptance

- `docker compose ps` shows Floci and the orchestrator; Aurora is a Floci container (`docker ps`, name `floci-aws-rds-*`). It does not show the job images.
- The web app: `curl -H "Host: $(terraform -chdir=infra/terraform output -raw frontend_domain)" http://127.0.0.1:11566/` returns the SPA `index.html` (200), and `/assets/<file>.js` returns the file. Deep links such as `/some/route` 404 on Floci because it does not run the CloudFront function.
- `POST /tradingcopilot.v1.ExperimentService/ListExperiments` with `Content-Type: application/json`, `Connect-Protocol-Version: 1`, body `{}` through the distribution returns 200 `{}`. An unknown method path returns the Lambda's own `404 page not found` (text/plain), not HTML.
- `task frontend:dev` is the UI for deep links and hot reload.
- A started experiment runs one `dagobert-backtest` container and ends with stored KPIs.

## Out of scope

Switching SQLite and local Parquet to Floci DynamoDB and S3. Fargate for the job containers (the migration task is the only Fargate task).
