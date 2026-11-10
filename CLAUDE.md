# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

Run from `devbox shell` (provides Go 1.24 and Task). Module: `github.com/MaxBernstetter/dagobert`.

```bash
task build              # job images + compose images
task build:jobs         # only the job images (ingest, technical indicators, backtest)
task up                 # compose stack: Floci, orchestrator; Terraform adds Aurora and the frontend (ECS) on :11001
task dev                # builds job and compose images, then task up
task frontend:dev       # Vite dev server with hot reload on the host (:3000)
task down               # stop stack; ./data is kept
task test               # system tests for the job images (builds each image, runs it)
```

Single system test (the job image must already be built):

```bash
go test ./tests/system/ingest -run TestName -count=1 -timeout 20m
```

CI (`.github/workflows/tests.yml`) runs all of this on every PR, every merge to main and nightly. Unit tests for shared packages run directly with `go test ./src/internal/...` and `go test ./src/shared/...`.

Frontend (`src/frontend`, Vite + React + TypeScript): `npx tsc --noEmit` is the typecheck (`npm run build` is `vite build` only); there is no lint or test script and no browser test suite. Run it locally with `npm run dev` (or `task frontend:dev`); the stack serves the production image from ECS.

Database and codegen: `task db:diff name=...` (Atlas, edit HCL in `db/`), `task db:migrate`, `task db:sqlc`, `task proto:gen` (`task proto:lint` and `task proto:breaking` before changing `proto/`).

## Architecture

**Data flow.** Dagobert is a backtesting framework. The orchestrator (`src/orchestrator`) hosts the Experiment API and starts stateless job containers through the Docker socket: `dagobert-ingest` and `dagobert-calculation-technical-indicator` (data backfill) and `dagobert-backtest` (runs an experiment: prepare, simulate, finalize). Exit codes are the contract: `0` succeeded, `1` failed, `2` aborted (treated as success, "already stored, nothing to do"). Each job logs `DETAIL=<n>` for diagnosis. There is no live or paper trading and no exchange credentials; fills are simulated.

**Shared state is files, not services.** The candle lake is Hive-partitioned Parquet in the Floci S3 bucket (`LAKE_BACKEND=s3`, the compose default; keys `parquet/provider=.../base=.../quote=.../year=.../candles.parquet`; `LAKE_BACKEND=fs` keeps it under `data/parquet`; `task lake:upload` copies an existing one). The indicator bars (`parquet/ti/...`, written by the indicator job and as a cache of experiment runs) live in the same bucket; there is no indicator database. Experiment workspace data (result shards and the prepare cache) is keyed by its path relative to `DATA_DIR` (`experiments/<id>/...`) and follows the same switch: files under `data/experiments/` with `LAKE_BACKEND=fs`, objects with the same keys in the bucket with `s3`, where nothing stays on the disk (`src/internal/workspace`; prepare, simulate, finalize, the orchestrator and the API share it through the bucket, so they need no shared disk). Everything else persists on the `./data` volume that the jobs and the orchestrator mount (the frontend does not): the legacy single-backtest catalog `backtests.sqlite` and `DATA_DIR` itself, which stays required as key root and scratch location. The experiment catalog, the ingest ledger (`ingest_stored`, `ingest_unavailable`: which candle hours are in the lake and which the exchange has none for) and the ingestion connector state (`provider`, `provider_pair`; secrets stay in `DATA_DIR/providers/secrets.json`) are Postgres (compose service, `DATABASE_URL`); row-heavy per-instance data is Parquet shards, one directory per persister flush, `data/experiments/<id>/shards/<shard>/` (the instance's `experiment_instance.shard_key`): `reports.parquet` (the report JSON), `fills`, `orders`, `journals`, `trade_outcomes`, `period_stats`, one file per datatype with one row group per instance (footer key `trading_copilot.instances` maps instance id to row group), written and read by `src/internal/instanceparquet` (a shard is uploaded as six objects after it is written; a reader downloads the whole file, a few MB, no range reads); the starting paper wallet of an instance is derived from the experiment settings at simulate and stored nowhere. Candle values are fixed-scale decimals (DECIMAL(18,8) prices, DECIMAL(28,8) volume), see `docs/ingest/binance-public-klines.md`; `src/internal/candlefile` reads both that and the old string schema, and `task data:migrate-candles` converts an existing lake. Path and layout helpers live in `src/shared/` (`datalayout.go`, `datasource.go`) and `src/internal/signalstore/`. Jobs run as `HOST_UID`/`HOST_GID` so bind mounts are not root-owned.

**Layout of `src/`.**
- `cmd/` + `internal/` + `runner/` per job (`ingest`, `calculation-technical-indicator`): each has a `cmd/<job>/main.go` entrypoint, `internal/job` for the stateless work, and `runner/` for the container-level orchestration (args, env, exit codes). Treat `runner/` as the boundary for the container contract.
- `calculation-technical-indicator/internal/indicators/<name>/` holds one indicator per package with a README; `internal/indicators/interface.go` is the contract and `run/` builds the dependency graph. `readme_sync_test.go` checks the READMEs against the code.
- `calculation-signal/` holds the playbooks and the replay framework (no container). A playbook is a directory under `playbooks/` (`strategy.go`, `DESCRIPTION.md`, tests); `internal/framework` defines the strategy/position/KPI model and `internal/playbooks/registry.go` maps names to implementations. `schema/playbook.schema.json` validates playbook config.
- `calculation-market-action/` sizes a signal against the simulated wallet (`internal/allocate`, called in-process via `runner.PlanAction`).
- `execution/` is the simulated order/fill layer (`runner.SimulateBar`, `internal/store`); `internal/venue/` holds the `paper` wallet and the `sim` cost model.
- `backtest/` + `cmd/backtest` run the framework over historical data for backtests and multi-instance experiments, using `internal/venue/sim`.
- `ingest/` fetches Binance public klines. Missing exchange hours are recorded as `unavailable`, not synthesized; coverage checks treat only retryable gaps as blocking.
- `orchestrator/` is the starter and CLI (`cmd/orchestrator`), including backfill and host data-dir handling.
- `experimentapi/` is the Experiment API, a Connect-RPC `ExperimentService` (`proto/tradingcopilot/v1/experiment_service.proto`, unary POST with protobuf or JSON bodies at `/tradingcopilot.v1.ExperimentService/`; experiments, instances, progress, reports); the orchestrator mounts it when `DATABASE_URL` is set. Each experiment can set its C++ process count ("Parallel processes", `parallelism`, auto = one per CPU). Experiments always use the C++ engine (`src/calculation`, ma-cross and signal-consensus only; `CPP_SMA_CROSS_BIN` and `CPP_SIGNAL_CONSENSUS_BIN` name the binaries). The sweep's actions are replayed in Go (`ReplayKPI`) with the simulated fill model (`SimulateBarWithModel`). Run `task cpp:deps` once, then `task cpp:test`.
- `frontend/` is the Vite + React SPA. It talks only to the Go Experiment API through the generated Connect client (`VITE_EXPERIMENT_API_URL`, `src/lib/experiment-api.ts`) and never reads `data/` directly. It does not start jobs. The Wiki reads the indicator and playbook docs (`src/wikiCatalog.ts`).

**Tests.** `tests/system/<job>/` builds and runs the real job image with a faked upstream (Binance HTTP, a seeded Parquet file). These tests are the proof of behaviour; they do not import service packages. Unit tests exist only for pure logic (`money`, `venue/sim`, indicator math, playbook logic). Per `docs/development/04-coding-principles.md`, do not add tests that lock internal call order or private types.

**Infra.** `infra/terraform` provisions the VPC, the Aurora PostgreSQL Serverless v2 cluster, the frontend ECS Fargate service (ECR image, public subnets), S3, SSM and IAM. The same code runs against Floci locally (`task infra:local`) and real AWS in CI only (`task infra:prod` refuses without `CI=true` and credentials). Aurora on Floci is the control-plane DB for local dev.

## Conventions and workflow

@AGENTS.md

Start with `docs/README.md` (index) and `docs/STATUS.md` (current state). Where project docs live: `docs/architecture/` (design), `docs/grooming/` (milestone notes; closed ones are archived), `docs/ingest/` (contracts). Wiki content lives only in the markdown files the Wiki renders (indicator `README.md`, playbook `DESCRIPTION.md`, strategy-KPI `README.md`); they are source code, see `docs/architecture/wiki-catalog.md`.
