# backtest

Runs experiments: many strategy instances over historical candles, using the playbook framework and the simulated venue. Entry point is `src/cmd/backtest` (container image `dagobert-backtest`, started by the orchestrator). Data flow: `docs/architecture/experiment-data-flow.md`; terms: `docs/glossary.md`.

## Phases
`ExperimentPhase()` (`experiment_phases.go`) picks one, from `EXPERIMENT_PHASE`:
- `prepare` (`RunExperimentPrepare`): load work, check candle coverage (`coverage.go`), ingest missing hours in-process, build indicators per symbol, write KPI bundles and meta to the workspace.
- `simulate` (`RunExperimentSimulate`): run instances, optionally a batch via `EXPERIMENT_INSTANCE_IDS`.
- `finalize` (`RunExperimentFinalize`): rebuild the experiment performance from each instance's `trade_outcomes.parquet` (bounded pool), set the final status, then remove the prepare cache (`.cache`). Re-finalizing a finished experiment needs a new run, because the cache is gone.
- default: `RunExperiment` runs all three in one process. Exit code is 0 or 1 only.

## Key files
- `run_experiment_phases.go`, `run_experiment_helpers.go`: the phases and shared helpers.
- `experiment_catalog.go`: the experiment catalog is Postgres (`pgcatalog`); `DATABASE_URL` is required and a run without it fails.
- `instance_files.go`: maps trade outcomes to the Parquet rows (`src/internal/instanceparquet`).
- `store/`: the legacy single-backtest catalog (`backtests.sqlite`, table `backtest`) and the shared experiment value types (`types.go`). The SQLite experiment catalog is gone.
- `run.go`: the older single-backtest path (`BACKTEST_ID`, `task backtest-run`).

## Engine
Experiments run on the C++ engine only (`cpp_engine.go`; the Go engine and `EXPERIMENT_ENGINE` were removed). Instances are grouped by symbol and playbook and sharded (`EXPERIMENT_CPP_SHARD_MAX`, default 500). `EXPERIMENT_CPP_PROCESSES` (default: one per CPU) is the number of concurrent `--sweep` processes. `cpp_sweep.go` builds the protobuf `SweepRequest`, runs the binary named by `CPP_SMA_CROSS_BIN` or `CPP_SIGNAL_CONSENSUS_BIN`, and `kpi_worker.go` (`calculation-signal/runner`) replays the actions in Go with `EXPERIMENT_SIM_WORKERS` workers (default: one per CPU). Only ma-cross and signal-consensus are supported; an unsupported playbook or a missing binary fails the instance with a clear error. The live pipeline does not use C++.

## Persistence
`persist_pipeline.go`: compute goroutines push results into a queue; one persister batches (up to 200 or 250 ms), writes `report.json` and the instance Parquet files (fills, orders, journals, trade outcomes; a failed instance removes them), then commits Postgres with retries and bisection down to a single instance. Postgres holds period stats, status, timing and metrics (COPY, `pgcatalog/instance_batch.go`). Compute never touches the database.

## Tests and benchmarks
- Unit tests: `go test ./src/backtest/...`. System tests: `tests/system/experiment` (needs Postgres, see `src/experimentapi/README.md`); experiments are created in the scratch database by `createExperimentPG`. The experiment tests need the C++ binaries (`CPP_SMA_CROSS_BIN`, `CPP_SIGNAL_CONSENSUS_BIN`).
- Replay consistency: `TestSmaCrossCppReplayConsistent`, `TestConsensusCppReplayConsistent` in `cpp_replay_test.go`; they skip without the binaries.
- The persistence benchmark: `TestBenchPersistGrid` (`tests/system/experiment/bench_persist_test.go`, `BENCH_INSTANCES`); see `docs/development/05-cpp-benchmark.md`.

## Before changing
The replay must keep the live fill model (`SimulateBarWithModel`) and the persisted shape the replay consistency tests check. Keep persistence out of the compute path.
