# Local stack

`task up` leaves three services running: Floci, the orchestrator and the frontend. Floci runs the Aurora database as a container it starts itself. It does not run anything by itself: experiments and backfills start when you ask for them.

## What stays up

| Process | Role |
|---|---|
| Frontend | The Vite SPA (`src/frontend`), published on `WEB_PORT` (default 11001). Reads through the Experiment API only; no `./data` mount. Does not start jobs. |
| Aurora (in Floci) | Aurora PostgreSQL Serverless v2, created by Terraform (`task infra:local`, from `infra/terraform`) and served by Floci on port 7001 (TLS, `sslmode=require`). Compose services reach it as `floci:7001`, host tools as `127.0.0.1:7001`. Catalog of experiments and instances, and the connector state (`provider`, `provider_pair`). The orchestrator mounts the Experiment API and the connector routes when `DATABASE_URL` is set. Secrets are not in Postgres. |
| Persistence | Candles: Hive-style S3 keys `parquet/provider={id}/base={B}/quote={Q}/year={YYYY}/candles.parquet` (footer metadata) in the Floci bucket (`LAKE_BACKEND=s3`, the compose default; `fs` keeps them under `./data/parquet`). Ingest and the indicator job (`parquet/ti/`) upload and keep no local copy; readers download to a temp file. The ingest ledger (`ingest_stored`, `ingest_unavailable`) is in Aurora; `task data:migrate-ingest-ledger` imports an old `ingest.sqlite`. The `./data` volume holds `technical_indicator.sqlite`, `backtests.sqlite` (legacy single-backtest catalog) and `data/experiments/<experiment_id>/` (per-instance workspaces). The jobs and the orchestrator mount this directory; the frontend does not. The experiment catalog and the connector state live in Postgres; `DATA_DIR/providers/secrets.json` holds the connector secrets. To move an existing `./data/parquet` lake into the bucket once, run `task lake:upload`. `task up` creates `data/parquet` on the host; the orchestrator ensures that layout before each job and runs job containers as `HOST_UID`/`HOST_GID` (from `task`) so bind mounts are not left root-owned. |
| Orchestrator | Floci on host port `FLOCI_PORT` (default 11566), and a starter process that owns the Docker socket. Also serves the Experiment API and the connector routes (`/api/v1/providers`, Connectors page) on `ORCHESTRATOR_PORT` (default 11090). The connector routes need `DATABASE_URL`; the Binance base URL is the saved config, else `BINANCE_BASE_URL`, else the public default. |

The job images (`dagobert-ingest`, `dagobert-calculation-technical-indicator`, `dagobert-backtest`) are runners. They appear when a backfill or an experiment needs them and exit when done.

The candle lake is in Floci S3 (bucket `dagobert-local-lake`, created by Terraform). The SQLite files and per-experiment workspaces still persist on `./data`.

## What the orchestrator does

1. On startup it checks the data layout and serves the HTTP API on `ORCHESTRATOR_PORT`: backfill, provider connectors and (with `DATABASE_URL`) the Experiment API.
2. A backfill or an experiment tells it which image to run and which arguments to pass. It runs that image, waits until it exits and reads the code. Exit `0` and exit `2` are success; exit `1` fails.
3. Data jobs run one at a time. Experiment fan-out is described in [experiment-data-flow.md](experiment-data-flow.md).

Floci's Lambda cannot see the host Docker socket. The orchestrator is the process that runs the images, with the socket mounted.

## Commands

| Command | Effect |
|---|---|
| `task build` | Job images plus compose images (orchestrator, production frontend). |
| `task build:jobs` / `task build:stack` | Build only the job images or only the compose services. |
| `task up` | `task db:migrate` (which applies Terraform on Floci first), then `docker compose up` with `compose.override.yaml` (dev frontend: bind-mount of `src/frontend` + Vite dev server). |
| `task dev` | `task build:jobs`, `task build:stack`, `task db:migrate`, then `docker compose up --watch` (same stack; Compose sync/rebuild on file changes). `db:up` also creates the roles `anon` and `authenticated` the migrations revoke from. |
| `task up:prod` | `COMPOSE_FILE=compose.yaml` only — production frontend image (`target: prod`). |
| `task up:build` | `task build` then `task up` (job images + compose build). |
| `task down` | Stops the stack. Does not delete `./data`. |

Dockerfiles use BuildKit cache mounts (`npm ci`, `go mod download`, `go build`, `npm ci` for the frontend) so rebuilds reuse work when only application source changed. The frontend image splits **deps** (lockfile) from **build** (source) so lockfile edits alone do not rerun `npm run build` on unrelated layers.

`compose.yaml` does not list the jobs; the orchestrator starts them. `task up`, `task up:build`, `task up:prod` and `task dev` apply Terraform (creates the VPC and the Aurora cluster, idempotent) and the Atlas migrations before starting compose. The Atlas CLI is not in the devbox shell.

## Acceptance

- `docker compose ps` shows Floci, the orchestrator and the web app. It does not show the job images.
- After `task db:migrate`, the web app at `WEB_PORT` lists experiments and can create one.
- A started experiment runs one `dagobert-backtest` container and ends with stored KPIs.

## Out of scope

Switching SQLite and local Parquet to Floci DynamoDB and S3. AWS Lambda and Fargate.
