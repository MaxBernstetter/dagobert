# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

Run from `devbox shell` (provides Go 1.24 and Task). Module: `github.com/MaxBernstetter/dagobert`.

```bash
task build              # job images
task build:jobs         # the job images (ingest, technical indicators, backtest)
task up                 # starts Floci (compose); Terraform adds Aurora, S3, the Lambdas, Step Functions, ECS, CloudFront and the web app upload; then migrations; prints the compiled web app URL (http://<id>.cloudfront.localhost:11566/)
task dev                # task up, then the web app (task frontend:dev) in the foreground on :3000
task frontend:dev       # Vite dev server with hot reload on the host (:3000)
task down               # stop stack; ./data is kept
task test               # all system tests against the running stack (task up first); each test gets its own bucket and scratch database
```

Single system test (the job image must already be built):

```bash
go test ./tests/system/ingest -run TestName -count=1 -timeout 20m
```

CI (`.github/workflows/tests.yml`) runs all of this on every PR, every merge to main and nightly. Unit tests for shared packages run directly with `go test ./src/internal/...` and `go test ./src/shared/...`.

Frontend (`src/frontend`, Vite + React + TypeScript): `npx tsc --noEmit` is the typecheck (`npm run build` is `vite build` only); there is no lint or test script and no browser test suite. Run it locally with `npm run dev` (or `task frontend:dev`); the deployed build is uploaded to S3 behind CloudFront by `scripts/frontend-deploy.sh`.

Database and codegen: `task db:diff name=...` (Atlas, edit HCL in `db/`), `task db:migrate`, `task db:sqlc`, `task proto:gen` (`task proto:lint` and `task proto:breaking` before changing `proto/`).

## Architecture

**Data flow.** Dagobert is a backtesting framework. Everything runs on AWS services; locally they are Floci's, created by Terraform (`infra/terraform`). The Experiment, Provider and Backfill Connect services are one Lambda (`src/cmd/experiment-api-lambda`) behind an API Gateway HTTP API. The same Lambda signs users in through Cognito (`POST /auth/login`, `src/internal/auth`) and answers 401 to every other call without a valid bearer token; local Terraform seeds the user `admin` / `password`. An experiment run starts the Step Functions execution `<prefix>-experiment-run`, which runs the worker Lambda (`src/cmd/experiment-worker`: prepare, simulate, finalize). A backfill starts `<prefix>-backfill`, which runs the ingest and indicator ECS tasks in order. The migrations run from the host (`task db:migrate`) or as an ECS task (`task db:migrate:ecs`). The job images `dagobert-ingest`, `dagobert-calculation-technical-indicator` and `dagobert-backtest` are the ECS task and sandbox images; `dagobert-backtest` runs only in the sandbox targets (`backtest-run`, `experiment-run`). Exit codes are the contract: `0` succeeded, `1` failed, `2` aborted (treated as success, "already stored, nothing to do"). Each job logs `DETAIL=<n>` for diagnosis. There is no live or paper trading and no exchange credentials; fills are simulated.

**Shared state is files, not services.** The candle lake is Hive-partitioned Parquet in the Floci S3 bucket (`LAKE_BACKEND=s3` on Floci and AWS, set by Terraform; keys `parquet/provider=.../base=.../quote=.../year=.../candles.parquet`; `LAKE_BACKEND=fs` (the default outside Terraform) keeps it under `data/parquet`; `task lake:upload` copies an existing one). The indicator bars (`parquet/ti/...`, written by the indicator job and as a cache of experiment runs) live in the same bucket; there is no indicator database. Experiment workspace data (result shards and the prepare cache) is keyed by its path relative to `DATA_DIR` (`experiments/<id>/...`) and follows the same switch: files under `data/experiments/` with `LAKE_BACKEND=fs`, objects with the same keys in the bucket with `s3`, where nothing stays on the disk (`src/internal/workspace`; prepare, simulate, finalize and the API share it through the bucket, so they need no shared disk). Everything else persists on the `./data` directory on the host, used by the sandbox targets: the legacy single-backtest catalog `backtests.sqlite` and `DATA_DIR` itself, which stays required as key root and scratch location. The experiment catalog, the ingest ledger (`ingest_stored`, `ingest_unavailable`: which candle hours are in the lake and which the exchange has none for) and the ingestion connector state (`provider`, `provider_pair`) is Postgres; the connector secrets are in AWS Secrets Manager at `/<project>/<env>/providers/<id>` (`SECRETS_BACKEND=file` keeps them in `DATA_DIR/providers/secrets.json`) (Aurora on Floci, `DATABASE_URL`); row-heavy per-instance data is Parquet shards, one directory per persister flush, `data/experiments/<id>/shards/<shard>/` (the instance's `experiment_instance.shard_key`): `reports.parquet` (the report JSON), `fills`, `orders`, `journals`, `trade_outcomes`, `period_stats`, one file per datatype with one row group per instance (footer key `trading_copilot.instances` maps instance id to row group), written and read by `src/internal/instanceparquet` (a shard is uploaded as six objects after it is written; a reader downloads the whole file, a few MB, no range reads); the starting paper wallet of an instance is derived from the experiment settings at simulate and stored nowhere. Candle values are fixed-scale decimals (DECIMAL(18,8) prices, DECIMAL(28,8) volume), see `docs/ingest/binance-public-klines.md`; `src/internal/candlefile` reads both that and the old string schema, and `task data:migrate-candles` converts an existing lake. Path and layout helpers live in `src/shared/` (`datalayout.go`, `datasource.go`) and `src/internal/signalstore/`.

**Layout of `src/`.**
- `cmd/` + `internal/` + `runner/` per job (`ingest`, `calculation-technical-indicator`): each has a `cmd/<job>/main.go` entrypoint, `internal/job` for the stateless work, and `runner/` for the container-level orchestration (args, env, exit codes). Treat `runner/` as the boundary for the container contract.
- `calculation-technical-indicator/internal/indicators/<name>/` holds one indicator per package with a README; `internal/indicators/interface.go` is the contract and `run/` builds the dependency graph. `readme_sync_test.go` checks the READMEs against the code.
- `calculation-signal/` holds the playbooks and the replay framework (no container). A playbook is a directory under `playbooks/` (`strategy.go`, `DESCRIPTION.md`, tests); `internal/framework` defines the strategy/position/KPI model and `internal/playbooks/registry.go` maps names to implementations. `schema/playbook.schema.json` validates playbook config.
- `calculation-market-action/` sizes a signal against the simulated wallet (`internal/allocate`, called in-process via `runner.PlanAction`).
- `execution/` is the simulated order/fill layer (`runner.SimulateBar`, `internal/store`); `internal/venue/` holds the `paper` wallet and the `sim` cost model.
- `backtest/` + `cmd/backtest` run the framework over historical data for backtests and multi-instance experiments, using `internal/venue/sim`.
- `ingest/` fetches Binance public klines. Missing exchange hours are recorded as `unavailable`, not synthesized; coverage checks treat only retryable gaps as blocking.
- `experimentapi/` is the Experiment API, a Connect-RPC `ExperimentService` (`proto/tradingcopilot/v1/experiment_service.proto`, unary POST with protobuf or JSON bodies at `/tradingcopilot.v1.ExperimentService/`; experiments, instances, progress, reports); the API Lambda mounts it (`src/cmd/experiment-api-lambda`). Each experiment can set its C++ process count ("Parallel processes", `parallelism`, auto = one per CPU). Experiments always use the C++ engine (`src/calculation`, ma-cross and signal-consensus only; `CPP_SMA_CROSS_BIN` and `CPP_SIGNAL_CONSENSUS_BIN` name the binaries). The sweep's actions are replayed in Go (`ReplayKPI`) with the simulated fill model (`SimulateBarWithModel`). Run `task cpp:deps` once, then `task cpp:test`.
- `frontend/` is the Vite + React SPA. It talks only to the Go Experiment API through the generated Connect client (`VITE_EXPERIMENT_API_URL`, `src/lib/experiment-api.ts`) and never reads `data/` directly. `task frontend:dev` proxies `/tradingcopilot.v1.` to the Floci API gateway (`scripts/local-api-url.sh`). It does not start jobs. The Wiki reads the indicator and playbook docs (`src/wikiCatalog.ts`).

**Tests.** `tests/system/<job>/` builds and runs the real job image with a faked upstream (Binance HTTP, a seeded Parquet file). These tests are the proof of behaviour; they do not import service packages. Unit tests exist only for pure logic (`money`, `venue/sim`, indicator math, playbook logic). Per `docs/development/04-coding-principles.md`, do not add tests that lock internal call order or private types.

**Infra.** `infra/terraform` provisions the VPC, the Aurora PostgreSQL Serverless v2 cluster, the web app (private S3 bucket and CloudFront distribution, `frontend.tf`), S3, SSM, Secrets Manager (provider secrets) and IAM. The same code runs against Floci locally (`task infra:local`) and real AWS in CI only (`task infra:prod` refuses without `CI=true` and credentials). Aurora on Floci is the control-plane DB for local dev.

## Conventions and workflow

@AGENTS.md

Start with `docs/README.md` (index) and `docs/STATUS.md` (current state). Where project docs live: `docs/architecture/` (design), `docs/grooming/` (milestone notes; closed ones are archived), `docs/ingest/` (contracts). Wiki content lives only in the markdown files the Wiki renders (indicator `README.md`, playbook `DESCRIPTION.md`, strategy-KPI `README.md`); they are source code, see `docs/architecture/wiki-catalog.md`.
