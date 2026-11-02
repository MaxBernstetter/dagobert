# orchestrator

One Go process (`src/orchestrator/cmd/orchestrator`) that fans out experiments, runs data backfills and hosts the Experiment API. It starts container jobs through the mounted Docker socket; jobs are stateless. See `docs/architecture/experiment-data-flow.md`.

## Experiment fan-out (`experiment_fanout.go`)
`runStudyJob` is chosen by `EXPERIMENT_RUN_BACKEND`:
- unset or `docker` (compose default): one `dagobert-backtest` container, `EXPERIMENT_PHASE=all`.
- `batched` or `sfn`: a `prepare` container, then `simulate` containers over batches of `EXPERIMENT_BATCH_SIZE` instances (default 64), then `finalize`. Batches run in a local pool bounded by `EXPERIMENT_MAP_MAX_CONCURRENCY` (default 8), or through Step Functions when `EXPERIMENT_SFN_ARN` is set. Any non-zero exit stops the experiment. Job state is in memory only.

## Data dir and backfill
- `host_data_dir.go`: job containers are started on the host, so bind mounts need the host path. `HOST_DATA_DIR` is required; a relative value is resolved, and the container's own mount is inspected as a fallback. Jobs run as `HOST_UID`/`HOST_GID`.
- `backfill.go`: `backfill` runs `dagobert-ingest` or `dagobert-calculation-technical-indicator` over a range, one at a time, and starts the HTTP server with the Experiment API (only with `DATABASE_URL`).
- `connectors.go`: market-data provider catalog (Binance) behind the Connectors page.

## Test and run
`go test ./src/orchestrator/...` (host path logic). The `task up` flow is the integration check.

## Before changing
The exit codes 0/1/2 are the contract with every job image. Do not add state outside `./data` and Postgres.
