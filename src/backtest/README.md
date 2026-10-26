# backtest

Runs experiments: many strategy instances over historical candles, using the same playbook and venue-simulation code as the live pipeline. Entry point is `src/cmd/backtest` (container image `dagobert-backtest`, started by the orchestrator). Data flow: `docs/architecture/experiment-data-flow.md`; terms: `docs/glossary.md`.

## Phases
`ExperimentPhase()` (`experiment_phases.go`) picks one, from `EXPERIMENT_PHASE`:
- `prepare` (`RunExperimentPrepare`): load work, check candle coverage (`coverage.go`), ingest missing hours in-process, build indicators per symbol, write KPI bundles and meta to the workspace.
- `simulate` (`RunExperimentSimulate`): run instances, optionally a batch via `EXPERIMENT_INSTANCE_IDS`.
- `finalize` (`RunExperimentFinalize`): write experiment KPIs and performance, set the final status.
- default: `RunExperiment` runs all three in one process. Exit code is 0 or 1 only.

## Key files
- `run_experiment_phases.go`, `run_experiment_helpers.go`: the phases and shared helpers.
- `experiment_catalog.go`: catalog is Postgres (`pgcatalog`) when `DATABASE_URL` is set, else SQLite `data/backtests.sqlite` (`store/`).
- `run.go`: the older single-backtest path (`BACKTEST_ID`, `task backtest-run`).

## Engines
`EXPERIMENT_ENGINE=go|cpp` (`cpp_engine.go`); default `go`, anything else fails. Go: `EXPERIMENT_SIM_WORKERS` (default 4) goroutines call `signalrunner.FastSimulatedPipelineOpts`. C++: instances are grouped by symbol and playbook and sharded (`EXPERIMENT_CPP_SHARD_MAX`, `EXPERIMENT_CPP_PROCESSES`); `cpp_sweep.go` builds the protobuf `SweepRequest`, runs the binary named by `CPP_SMA_CROSS_BIN` or `CPP_SIGNAL_CONSENSUS_BIN`, and replays the actions in Go (`kpi_worker.go` in `calculation-signal/runner`). Only ma-cross and signal-consensus are supported; there is no Go fallback, an unsupported playbook or missing binary fails the instance. The live pipeline does not use C++.

## Persistence
`persist_pipeline.go`: compute goroutines push results into a queue; one persister batches (up to 200 or 250 ms), writes `report.json` files, then commits with retries and bisection down to a single instance. Postgres writes fills and stats via COPY (`pgcatalog/instance_batch.go`); SQLite stores only status, timing and metrics, so the two catalogs are not comparable. Compute never touches the database.

## Tests and benchmarks
- Unit tests: `go test ./src/backtest/...`. System tests: `tests/system/experiment` (see `tests/README.md`).
- Go-vs-C++ equivalence: `TestSmaCrossGoVsCppEquivalence`, `TestConsensusGoVsCppEquivalence` in `cpp_sweep_test.go`; skipped unless the binary is built (`task cpp:build`) or its `CPP_*_BIN` is set.
- `bench_test.go` (`BenchmarkGoExperiment`, needs `BENCH_LAKE_DIR`) and `task cpp:bench`; see `docs/development/05-cpp-benchmark.md`.

## Before changing
Go and C++ must produce the same results: change both or update the equivalence tests. Keep persistence out of the compute path.
