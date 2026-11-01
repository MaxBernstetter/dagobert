# Local stack

`task up` leaves four services running: Floci, Postgres, the orchestrator and the frontend. It should not execute a candle by itself.

## What stays up

| Process | Role |
|---|---|
| Frontend | The Vite SPA (`src/frontend`), published on `WEB_PORT` (default 11001). Reads through the Experiment API only; no `./data` mount. Does not start jobs. |
| Postgres | `postgres:16-alpine` on `POSTGRES_PORT` (default 11432), volume `postgres_data`. Catalog of experiments and instances, and the connector state (`provider`, `provider_pair`). The orchestrator mounts the Experiment API and the connector routes when `DATABASE_URL` is set. Secrets are not in Postgres. |
| Persistence | The `./data` volume: Hive-style `parquet/provider={id}/base={B}/quote={Q}/year={YYYY}/candles.parquet` (footer metadata) plus `ingest.sqlite`, `technical_indicator.sqlite`, `calculation_signal.sqlite`, `execution.sqlite`, `backtests.sqlite` (legacy single-backtest catalog) and `data/experiments/<experiment_id>/` (per-instance workspaces). The jobs and the orchestrator mount this directory; the frontend does not. The experiment catalog and the connector state live in Postgres; `DATA_DIR/providers/secrets.json` holds the connector secrets. `task up` creates `data/parquet` on the host; the orchestrator ensures that layout before each job and runs job containers as `HOST_UID`/`HOST_GID` (from `task`) so bind mounts are not left root-owned. |
| Orchestrator | Floci on host port `FLOCI_PORT` (default 11566), and a starter process that owns the Docker socket. Also serves the Experiment API on `ORCHESTRATOR_PORT` (default 11090). |

The five job images are runners. They appear when a candle close is due and exit when that stage is done.

Floci DynamoDB and S3 are not used for job state. The jobs persist on `./data`; pointing them at Floci is out of scope for now.

## What the orchestrator does

1. On startup the starter registers one state machine: ingest → calculation-technical-indicator → calculation-signal → calculation-market-action → execution. The input is the candle close in milliseconds.
2. Each state tells the starter which image to run and which arguments to pass. The starter runs that image, waits until it exits, and returns the code. Exit `0` and exit `2` are success. Exit `1` fails the state. `DETAIL` stays on the container log. Bybit credentials go only to the execution container.
3. Retry is on the state, three attempts, and only after exit `1`. A later state does not start until the current one has exited `0` or `2`.
4. A clock starts one execution at each UTC boundary: 00:00, 03:00, 06:00, 09:00, 12:00, 15:00, 18:00, 21:00. The input is that boundary. If Floci cannot express the schedule, the starter keeps that clock itself. One clock, not both.
5. A second execution of the same close is a new execution. Each job no-ops (`DETAIL=1`).

Floci's Lambda cannot see the host Docker socket. The starter is the process that runs the images. It can live in the compose network with the socket mounted.

## Commands

| Command | Effect |
|---|---|
| `task build` | Job images plus compose images (orchestrator, production frontend). |
| `task build:jobs` / `task build:stack` | Build only pipeline jobs or only compose services. |
| `task up` | `docker compose up` with `compose.override.yaml` (dev frontend: bind-mount of `src/frontend` + Vite dev server). |
| `task dev` | `task build:jobs`, `task db:migrate`, then `docker compose up --watch` (same stack; Compose sync/rebuild on file changes). `db:up` also creates the Supabase roles `anon` and `authenticated` the migrations revoke from. |
| `task up:prod` | `COMPOSE_FILE=compose.yaml` only — production frontend image (`target: prod`). |
| `task up:build` | `task build` then `task up` (job images + compose build). |
| `task run` | One execution for `CANDLE_CLOSE_MS`. Default `1700006400000`. |
| `task down` | Stops the stack. Does not delete `./data`. |

Dockerfiles use BuildKit cache mounts (`npm ci`, `go mod download`, `go build`, `npm ci` for the frontend) so rebuilds reuse work when only application source changed. The frontend image splits **deps** (lockfile) from **build** (source) so lockfile edits alone do not rerun `npm run build` on unrelated layers.

`compose.yaml` does not list the jobs; the orchestrator starts them. Nothing applies the Postgres migrations automatically; run `task db:migrate` after the first `task up` (needs the Atlas CLI, which is not in the devbox shell) and after new migrations.

## Acceptance

- `docker compose ps` shows Floci, Postgres, the starter, and the web app. It shows Floci, Postgres, the starter and the web app. It does not show the five jobs.
- `task run` for a new close runs the five images in order. The app shows the action and, when an order is sent, the fill and the balances.
- `task run` for a close that is already stored prints `DETAIL=1` for each job, sends no second order, and the execution succeeds.
- A job that exits `1` fails that state. The retry does not start the earlier jobs again.

## Out of scope

Switching SQLite and local Parquet to Floci DynamoDB and S3. AWS EventBridge, Lambda, and Fargate. Backfill. The pipeline page reading Step Functions history.
