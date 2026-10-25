# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

Run from `devbox shell` (provides Go 1.24 and Task). Module: `github.com/MaxBernstetter/dagobert`.

```bash
task build              # job images + compose images
task build:jobs         # only the pipeline job images (dagobert-<job>)
task up                 # compose stack: Floci, orchestrator, dev frontend on :3001
task dev                # same, with compose watch
task down               # stop stack; ./data is kept
task run                # one candle-close execution (CANDLE_CLOSE_MS, default 1700006400000)
task test               # system tests for the job images (builds each image, runs it)
```

Single system test (the job image must already be built):

```bash
go test ./tests/system/ingest -run TestName -count=1 -timeout 20m
```

Unit tests for shared packages run directly with `go test ./src/internal/...` and `go test ./src/shared/...`.

Frontend (`src/frontend`, Vite + React + TypeScript): `npm run build` is the typecheck gate; there is no lint or test script and no browser test suite. Run it locally with `npm run dev`, or in the container via `task dev`.

Database and codegen: `task db:diff name=...` (Atlas, edit HCL in `db/`), `task db:migrate`, `task db:sqlc`, `task proto:gen` (`task proto:lint` and `task proto:breaking` before changing `proto/`).

## Architecture

**Data flow.** The orchestrator (`src/orchestrator`) owns a state machine that runs five stateless job containers per 3-hour UTC candle: ingest → calculation-technical-indicator → calculation-signal → calculation-market-action → execution. The starter process mounts the Docker socket and runs each image. Exit codes are the contract: `0` succeeded, `1` failed (retried up to three times on that state), `2` aborted (treated as success, used for "already stored, nothing to do"). Each job logs `DETAIL=<n>` for diagnosis. Re-running a close is a new execution, and every job no-ops on work already stored, so only the first run places an order.

**Shared state is files, not services.** Everything persists on the `./data` volume that both the jobs and the frontend mount: Hive-partitioned Parquet under `data/parquet/provider=.../base=.../quote=.../year=.../candles.parquet`, plus SQLite files (`ingest.sqlite`, `technical_indicator.sqlite`, `calculation_signal.sqlite`, `execution.sqlite`, `backtests.sqlite`). Path and layout helpers live in `src/shared/` (`datalayout.go`, `datasource.go`) and `src/internal/signalstore/`. Jobs run as `HOST_UID`/`HOST_GID` so bind mounts are not root-owned.

**Layout of `src/`.**
- `cmd/` + `internal/` + `runner/` per job: each job has a `cmd/<job>/main.go` entrypoint, `internal/job` for the stateless work, and `runner/` for the container-level orchestration (args, env, exit codes). Treat `runner/` as the boundary for the container contract.
- `calculation-technical-indicator/internal/indicators/<name>/` holds one indicator per package with a README; `internal/indicators/interface.go` is the contract and `run/` builds the dependency graph. `readme_sync_test.go` checks the READMEs against the code.
- `calculation-signal/` runs playbooks. A playbook is a directory under `playbooks/` (`strategy.go`, `DESCRIPTION.md`, tests); `internal/framework` defines the strategy/position/KPI model and `internal/playbooks/registry.go` maps names to implementations. `schema/playbook.schema.json` validates playbook config.
- `calculation-market-action/` sizes the signal against live Bybit balances (`internal/allocate`) and writes the action.
- `execution/` is the only container with Bybit credentials. It places the paper order on the Bybit EU demo host and stores the fill; `internal/venue/` holds the venue clients (`bybit`, `paper`, `sim`).
- `backtest/` + `cmd/backtest` run the same framework over historical data for backtests and multi-instance experiments. Simulation uses `internal/venue/sim` and the same playbook code as the live pipeline, so results match production logic.
- `ingest/` fetches Binance public klines. Missing exchange hours are recorded as `unavailable`, not synthesized; coverage checks treat only retryable gaps as blocking.
- `orchestrator/` is the starter and CLI (`cmd/orchestrator`), including backfill and host data-dir handling.
- `frontend/` is the Vite + React SPA. It talks only to the Go Experiment API (`VITE_EXPERIMENT_API_URL`) and never reads `data/` directly. It does not start jobs. The Wiki reads the indicator and playbook docs (`src/wikiCatalog.ts`).

**Tests.** `tests/system/<job>/` builds and runs the real job image with a faked upstream (Binance HTTP, a seeded Parquet file or database row, a fake Bybit response). These tests are the proof of behaviour; they do not import service packages. Unit tests exist only for pure logic (`money`, `venue/sim`, indicator math, playbook logic). Per `docs/development/04-coding-principles.md`, do not add tests that lock internal call order or private types.

**Infra.** `infra/terraform` provisions S3/SSM/IAM against Floci locally (`task infra:local`, then `task infra:seed-ssm` to push `.secrets` into SSM), and Supabase for prod (`task infra:prod`). Compose Postgres is the control-plane DB for local dev.

## Conventions and workflow

@AGENTS.md

Where project docs live: `docs/architecture/` (design), `docs/grooming/` (milestone notes; closed ones are archived), `docs/indicators/` and `docs/ingest/` (contracts).
