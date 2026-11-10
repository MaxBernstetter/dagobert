# experimentapi

The Connect-RPC API behind the experiments UI, backed by Postgres. It creates experiments and instances, starts and cancels runs, and serves progress, instances, reports, charts and performance. Data flow: `docs/architecture/experiment-data-flow.md`.

## Service
`ExperimentService` (`proto/tradingcopilot/v1/experiment_service.proto`), mounted by `Mount` in `api.go` at `/tradingcopilot.v1.ExperimentService/`. Every method is a unary HTTP POST (Connect protocol) with a protobuf or JSON body: ListExperiments, GetExperiment, CreateExperiment (manual list or grid), DeleteExperiment, GetExperimentStatus, GetExperimentProgress, ListStrategies, GetExperimentLimits (`maxParallelism`, the largest "Parallel processes" value the create form accepts), GetExperimentPerformance, ListInstances, GetInstance, GetChartSeries, GetInstanceReport, GetInstanceKpis, GetInstanceActivity, RunExperiment, CancelExperiment. The SPA uses a generated Connect client with binary protobuf (`src/frontend/src/lib/experiment-api.ts`).

```bash
curl -X POST -H 'Content-Type: application/json' -d '{}' \
  http://127.0.0.1:11090/tradingcopilot.v1.ExperimentService/ListExperiments
curl -X POST -H 'Content-Type: application/json' -d '{"id":"<experiment-id>"}' \
  http://127.0.0.1:11090/tradingcopilot.v1.ExperimentService/GetExperimentProgress
```
JSON uses camelCase field names, int64 as strings. Errors are Connect errors (`{"code":"not_found","message":"..."}`): a missing experiment, instance, report or KPIs is `not_found`; a malformed id or invalid create argument `invalid_argument`; delete of a running or materializing experiment, run of a running one, cancel of a non-running one and a retry without failed instances `failed_precondition`; anything else `internal`.

## ProviderService
`ProviderService` (`proto/tradingcopilot/v1/provider_service.proto`), mounted by the same `Mount` at `/tradingcopilot.v1.ProviderService/`: ListProviders, GetProvider, SaveProviderSettings (config and write-only secrets), ActivateProvider (runs the setup before it answers: the view is ready or failed), DeactivateProvider, ListProviderPairs. The logic is `providers/` (`service.go`: state, setup, pairs, base URL; `secrets.go`: the secret backends). Secret values never leave the service; a view lists only the names of the secrets that are set. The Connectors page calls it through the generated client (`src/frontend/src/lib/provider-service.ts`). The orchestrator's REST routes (`/api/v1/providers...`) are deprecated and share the same `providers.Service`.

Secret backend (`SECRETS_BACKEND`): `secretsmanager` stores one secret per provider at `/<project>/<env>/providers/<id>` (`SSM_PREFIX`), a JSON object of key to value; `file` (alias `fs`) keeps `DATA_DIR/providers/secrets.json` (mode 0600). `auto` (default) picks `secretsmanager` when `AWS_ENDPOINT_URL` or `SSM_PREFIX` is set, else `file`.

## Parallel processes
`settings.parallelism` (proto `ExperimentSettings.parallelism`, and `parallelism` on the experiment) is the number of C++ sweep processes of an experiment. Unset or 0 means auto: one per CPU of the worker. It is optional in the proto, so a response without a value leaves `parallelism` unset, as before. Create rejects a negative value or a value above the maximum with `invalid_argument` and a message that says so. The maximum is the CPU count of the API process, or `EXPERIMENT_MAX_PARALLELISM` when set. The worker reads the stored value at each run, so a retry or re-run uses the same count.

## Response encoding and budgets
- Gzip: Connect compresses a response with gzip when the request has `Accept-Encoding: gzip` (browsers send it); chart series and activity rows are large.
- `GetChartSeries` (`fromSec`, `toSec`, optional `maxPoints`; returns a `ChartSeries`): with `maxPoints` above 0 and more candles in the window than that, the candles are grouped into at most N consecutive buckets of equal size (the last may be smaller). Bucket: open of the first candle, high and low over the bucket, close of the last, time of the first. Each indicator takes its value at the bucket's last candle. Portfolio and markers are not downsampled. Absent, 0 or negative keeps every candle. A window with `toSec` not above 0 or `fromSec` above `toSec` is `invalid_argument`. The unbudgeted response is unchanged. Bucketing is `candleBuckets` and `downsampleSeries` in `chart.go`.

## Layout
- `handlers.go` (service methods, error helpers), `create.go`, `grid.go`: list/create/run/cancel; creation materializes instances asynchronously (status `materializing`, then `draft`).
- `instance_detail.go`, `instance_files.go`, `chart.go`, `performance_api.go`, `status.go`, `strategies.go`: read methods. Instance activity, chart series and trade outcomes are read from the instance's row groups in its shard (`experiment_instance.shard_key`, `src/internal/instanceparquet`), as is the report; an instance without `shard_key`, a missing file or a missing row group is an empty result (report and KPIs: `not_found`), not an error. Shard files are read through `src/internal/workspace` (a local file, or with `LAKE_BACKEND=s3` a download of the whole object per read, a known cost for shards of a few MB). Experiment delete removes `experiments/<id>/` from the disk or the bucket. `workspace_rel_path` is no longer read here.
- `pgcatalog/`: Postgres catalog used by the backtest worker too (`Open` keys on `DATABASE_URL`); `instance_batch.go` persists instance status, timing and metrics. `performance.go` rebuilds the experiment performance from the outcome and period stat files at finalize.
- `experimentdb/`: sqlc-generated queries. Do not edit by hand; see `db/README.md`.
- `persist/`, `map.go`: mapping between rows and protobuf messages.

## Who mounts it
- The orchestrator mounts it in-process when `DATABASE_URL` is set (`src/orchestrator/cmd/orchestrator/backfill.go`) and supplies `Hooks`: `RunExperiment`, `CancelStudy`, `JobStatus`. A run request marks rows running and then calls the hook. Its REST provider routes use `Server.Providers()`, the same service instance.
- The API Lambda (`src/cmd/experiment-api-lambda`) mounts both services; its secrets come from Secrets Manager (`infra/terraform/api_lambda.tf`).
- `src/cmd/experiment-api` is a standalone server with empty hooks, so it cannot start runs.
- `src/cmd/experiment-api-lambda` also serves `BackfillService` (`backfill.go`, validation and execution mapping in `backfill_plan.go`) when `BACKFILL_SFN_ARN` is set. It starts, describes, lists and stops executions of the backfill state machine, and reads provider state from the same Postgres pool.

## Test and run
`go test ./src/experimentapi/...` covers pure logic (grid, chart, metrics, provider base URL, secret mapping) and the transport (protobuf, JSON, gzip). `tests/system/provider` runs ProviderService against Floci Secrets Manager and a scratch Postgres (`POSTGRES_TEST_URL`). Behaviour is proven by `tests/system/experiment`, which needs a Postgres 16 reachable at `POSTGRES_TEST_URL` (default: the compose Postgres on 9432); the tests fail, not skip, without it. Locally: `task up` (compose Postgres, migrations via `task db:migrate`).

## Before changing
A new query goes in `db/sqlc/queries`, then `task db:sqlc`. A schema change goes through Atlas (`db/README.md`). Response shapes come from `proto/`; the frontend reads them through the generated client (`task proto:gen`). `ErrorResponse` and `OkResponse` in `experiment.proto` are unused since the REST routes went; removing them is a breaking change, left for later.
