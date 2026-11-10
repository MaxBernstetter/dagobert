# Local stack

`task up` starts one compose service, Floci. Floci runs everything the system needs, created by the Terraform in `infra/terraform`: Aurora, the S3 buckets, SSM, Secrets Manager, the Experiment API Lambda behind an API Gateway HTTP API, the experiment worker Lambda, the Step Functions state machines, the ECS job tasks and the web app's CloudFront distribution. No process outside Floci runs the system. Experiments and backfills start when you ask for them, through the API.

## What runs where

| Component | Where | Notes |
|---|---|---|
| Web app (S3 + CloudFront in Floci) | Bucket `dagobert-local-frontend`, distribution in `frontend.tf` | The production SPA (`src/frontend`), built and uploaded by `scripts/frontend-deploy.sh local`. CloudFront routes `/tradingcopilot.v1.*` to the API gateway. Served at the `frontend_domain` output through Floci's CloudFront (Host header). Floci does not run the SPA-fallback function, so deep links 404 locally. For hot reload and deep links use `task frontend:dev`. |
| Aurora (in Floci) | Aurora PostgreSQL Serverless v2, created by Terraform (`task infra:local`), served by Floci on port 7001 (TLS, `sslmode=require`) | Experiment catalog, ingest ledger (`ingest_stored`, `ingest_unavailable`), connector state (`provider`, `provider_pair`). Compose services reach it as `floci:7001`, host tools as `127.0.0.1:7001`. `task data:migrate-ingest-ledger` imports an old `ingest.sqlite`. Secrets are not in Postgres. |
| Candle lake | Floci S3 bucket `dagobert-local-lake` (`LAKE_BACKEND=s3`) | Hive-style keys `parquet/provider={id}/base={B}/quote={Q}/year={YYYY}/candles.parquet`. Ingest and the indicator job (`parquet/ti/`) upload and keep no local copy. `task lake:upload` moves an existing `./data/parquet` lake into the bucket once. |
| Experiment workspace | Same bucket, `experiments/<id>/...` (`LAKE_BACKEND=s3`) | Result shards and the prepare cache. With `LAKE_BACKEND=fs` they live under `./data/experiments/` (`src/internal/workspace`). |
| Secrets | Secrets Manager `/dagobert/local/providers/<id>` | Connector secrets. `SECRETS_BACKEND=file` keeps them in `DATA_DIR/providers/secrets.json`. |
| Experiment API | API Gateway HTTP API, Lambda `<prefix>-experiment-api` (`api_lambda.tf`) | Serves `ExperimentService`, `ProviderService` and `BackfillService` (`POST {url}/tradingcopilot.v1.<Service>/<Method>`). On Floci the output host does not resolve; use `http://127.0.0.1:11566/_aws/execute-api/<api-id>/$default/...`. `scripts/local-api-url.sh` prints that origin and prefix for the dev server. |
| Experiment run | Step Functions `<prefix>-experiment-run` (`experiment_workflow.tf`), worker Lambda `<prefix>-experiment-worker` | Run, Cancel and progress start, stop and describe the execution (`EXPERIMENT_SFN_ARN`). The worker runs the prepare, simulate and finalize phases. |
| Backfill | Step Functions `<prefix>-backfill` (`backfill_sfn.tf`), ECS Fargate tasks (`jobs_ecs.tf`) | See [Backfill on ECS](#backfill-on-ecs). |
| Database migrations | Atlas, from the host (`task db:migrate`) or as a one-off ECS task (`task db:migrate:ecs`) | Both apply the same `db/` migrations. |
| Data directory | `./data` on the host | `backtests.sqlite` (legacy single-backtest catalog), `DATA_DIR` scratch, and the output of the sandbox targets (`task backtest-run`, `task experiment-run`, `task seed-demo-year`). Nothing in the Floci stack mounts it. |

The job images (`dagobert-ingest`, `dagobert-calculation-technical-indicator`, `dagobert-backtest`) are runners. The ECS tasks and the worker run their own images from ECR, pushed by `scripts/infra-apply.sh`. `dagobert-backtest` runs only in the sandbox targets below.

## Commands

| Command | Effect |
|---|---|
| `task up` | Creates `data/parquet`, starts Floci (`docker compose up -d --wait floci`), then `task db:migrate`: Terraform apply on Floci, the roles the migrations revoke from, Atlas migrations, web app upload. |
| `task up:build` | `task build` (job images), then `task up`. |
| `task dev` | `task up`, then a reminder to run `task frontend:dev`. |
| `task frontend:dev` | Vite dev server with hot reload on the host (port 3000). Proxies `/tradingcopilot.v1.` to the Floci API gateway: `scripts/local-api-url.sh` sets `VITE_DEV_API_PROXY` (origin) and `VITE_DEV_API_PREFIX` (`/_aws/execute-api/<api-id>/$default`). Run `task up` first. |
| `task down` | Stops Floci. Does not delete `./data`. |
| `task build` / `task build:jobs` | Build the job images (ingest, technical indicators, backtest). |
| `task build:api-lambda` | Build the Experiment API Lambda image locally. `scripts/infra-apply.sh` builds it again under the ECR URI. |
| `task db:migrate:ecs` | Runs the Atlas migrations as a one-off ECS Fargate task (`scripts/db-migrate.sh local`). Waits for the task and fails unless the container exits 0. Floci's ECS and exit-code reporting are not verified yet. |
| `task seed-demo-year` | Wipes `./data`, backfills a year of BTC in-process and replays `ma-cross` with the simulated venue. Needs `DATABASE_URL`. |
| `task backtest-run`, `task experiment-run` | Sandbox: `docker run` of `dagobert-backtest` against `./data`, no Step Functions. |

Dockerfiles use BuildKit cache mounts (`npm ci`, `go mod download`, `go build`) so rebuilds reuse work when only application source changed. The SPA is not in a Docker image: `scripts/frontend-deploy.sh` builds it and uploads `dist/` to S3. The Atlas CLI is not in the devbox shell.

## Acceptance

- `docker compose ps` shows only `floci`. Aurora is a Floci container (`docker ps`, name `floci-aws-rds-*`). It does not show the job images.
- The web app: `curl -H "Host: $(terraform -chdir=infra/terraform output -raw frontend_domain)" http://127.0.0.1:11566/` returns the SPA `index.html` (200), and `/assets/<file>.js` returns the file. Deep links such as `/some/route` 404 on Floci because it does not run the CloudFront function.
- `POST /tradingcopilot.v1.ExperimentService/ListExperiments` with `Content-Type: application/json`, `Connect-Protocol-Version: 1`, body `{}` through the distribution returns 200 `{}`. An unknown method path returns the Lambda's own `404 page not found` (text/plain), not HTML.
- `task frontend:dev` is the UI for deep links and hot reload.
- A started experiment runs the Step Functions execution `<prefix>-experiment-run` and ends with stored KPIs.

## Backfill on ECS

Ingest and the indicator backfill run as ECS Fargate tasks (`infra/terraform/jobs_ecs.tf`), driven by the Step Functions state machine `<prefix>-backfill` (`backfill_sfn.tf`). Backfills start through `BackfillService` (`proto/tradingcopilot/v1/backfill_service.proto`), served by the Experiment API Lambda (`BACKFILL_SFN_ARN`, `src/experimentapi/backfill*.go`). It validates the request, starts one execution per market and reports status, list and cancel.

- Apply: `task infra:local` creates the repositories, builds and pushes `dagobert-local-ingest` and `dagobert-local-indicator` images under their ECR URIs, then creates the state machine.
- Start through the API: `BackfillService/StartBackfill` with `{"provider":"binance","symbols":["BTCUSDT"],"fromMs":"1704067200000","toMs":"1704153600000"}` (Connect unary POST to `/tradingcopilot.v1.BackfillService/StartBackfill`). `GetBackfillStatus` takes the returned `executionArn`. The provider must be activated on the Connectors page first.
- Direct start and wait: `scripts/backfill-start.sh local '{"provider":"binance","base":"BTC","quote":"USDT","from":1704067200000,"to":1704153600000}'`. The script finds the state machine by name, starts one execution and prints its output. Exit 0 means SUCCEEDED.
- Exit codes: the ECS job containers set `COMPOSE_OK_ON_ABORT=1`, so exit 2 (already stored, nothing to do) exits 0 and the task succeeds; the state machine (`runTask.sync`) continues on exit 0. Exit 1 fails the Task state and ends the execution FAILED.
- Verified on Floci: `runTask.sync` runs the tasks, and they write the lake without `AWS_ENDPOINT_URL` set on the task. Real AWS is the target and is not run here.

## Out of scope

Switching SQLite and local Parquet to Floci DynamoDB and S3. A UI for BackfillService (none yet; the Connectors page does not start backfills).
