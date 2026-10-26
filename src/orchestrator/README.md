# orchestrator

One Go process (`src/orchestrator/cmd/orchestrator`) that runs the live pipeline, fans out experiments and hosts the Experiment API. It starts job containers through the mounted Docker socket; jobs are stateless. See `docs/architecture/pipeline-data-flow.md` and `docs/architecture/experiment-data-flow.md`.

## Live pipeline (`main.go`)
`serve()` registers a Step Functions state machine on Floci with five task states (ingest, calculation-technical-indicator, calculation-signal, calculation-market-action, execution). Each has a Retry on `JobFailed`, `MaxAttempts` 2 (so up to three attempts overall). A clock starts one execution per 3-hour UTC candle boundary. `runJob` runs the job image: exit 0 and 2 succeed, 1 fails. `orchestrator run` (`task run`) does one execution for `CANDLE_CLOSE_MS`.

## Experiment fan-out (`experiment_fanout.go`)
`runStudyJob` is chosen by `EXPERIMENT_RUN_BACKEND`:
- unset or `docker` (compose default), or `EXPERIMENT_RUN_HEAVY=1`: one `dagobert-backtest` container, `EXPERIMENT_PHASE=all`.
- `batched` or `sfn`: a `prepare` container, then `simulate` containers over batches of `EXPERIMENT_BATCH_SIZE` instances (default 64), then `finalize`. Batches run in a local pool bounded by `EXPERIMENT_MAP_MAX_CONCURRENCY` (default 8), or through Step Functions when `EXPERIMENT_SFN_ARN` is set. Any non-zero exit stops the experiment. Job state is in memory only.

## Data dir and backfill
- `host_data_dir.go`: job containers are started on the host, so bind mounts need the host path. `HOST_DATA_DIR` is required; a relative value is resolved, and the container's own mount is inspected as a fallback. Jobs run as `HOST_UID`/`HOST_GID`.
- `backfill.go`: `backfill` re-runs ingest over a range, and starts the HTTP server with the Experiment API (only with `DATABASE_URL`).

## Test and run
`go test ./src/orchestrator/...` (host path logic). The `task up` and `task run` flows are the integration check.

## Before changing
The exit codes 0/1/2 are the contract with every job image. Do not add state outside `./data` and Postgres.
