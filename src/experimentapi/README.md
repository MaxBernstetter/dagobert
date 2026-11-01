# experimentapi

The HTTP API behind the experiments UI, backed by Postgres. It creates experiments and instances, starts and cancels runs, and serves progress, instances, reports, charts and performance. Data flow: `docs/architecture/experiment-data-flow.md`.

## Routes
All under `/api/v1/experiments`, registered in `api.go` (`Mount`): list, `strategies`, create (manual list or grid), get, delete, `{id}/status`, `{id}/progress`, `{id}/performance`, `{id}/instances`, `{id}/instances/{instanceId}` plus `/chart-series`, `/report`, `/kpis`, `/activity`, and `POST {id}/run` and `{id}/cancel`.

## Layout
- `handlers.go`, `create.go`, `grid.go`: list/create/run/cancel; creation materializes instances asynchronously (status `materializing`, then `draft`).
- `instance_detail.go`, `instance_files.go`, `chart.go`, `performance_api.go`, `status.go`, `strategies.go`: read endpoints. Instance activity, chart series and trade outcomes are read from the instance Parquet files (`src/internal/instanceparquet`); a missing file is an empty result, not an error.
- `pgcatalog/`: Postgres catalog used by the backtest worker too (`Open` keys on `DATABASE_URL`); `instance_batch.go` persists instance status, timing, metrics and period stats (COPY). `performance.go` rebuilds the experiment performance from the outcome files at finalize.
- `experimentdb/`: sqlc-generated queries. Do not edit by hand; see `db/README.md`.
- `persist/`, `protojson.go`, `map.go`: mapping between rows and protobuf messages.

## Who mounts it
- The orchestrator mounts it in-process when `DATABASE_URL` is set (`src/orchestrator/cmd/orchestrator/backfill.go`) and supplies `Hooks`: `RunExperiment`, `CancelStudy`, `JobStatus`. A run request marks rows running and then calls the hook.
- `src/cmd/experiment-api` is a standalone server with empty hooks, so it cannot start runs.

## Test and run
`go test ./src/experimentapi/...` covers pure logic (grid, chart, metrics). Behaviour is proven by `tests/system/experiment`, which needs a Postgres 16 reachable at `POSTGRES_TEST_URL` (default: the compose Postgres on 9432); the tests fail, not skip, without it. Locally: `task up` (compose Postgres, migrations via `task db:migrate`).

## Before changing
A new query goes in `db/sqlc/queries`, then `task db:sqlc`. A schema change goes through Atlas (`db/README.md`). Response shapes come from `proto/`; the frontend reads them via `src/frontend/src/api.ts`.
