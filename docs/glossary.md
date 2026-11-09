# Glossary

Alphabetical. Each entry names where to look in the code.

- **action**: the sized decision derived from a signal. `src/calculation-market-action/`. Experiments do not store actions; their orders are in the instance `orders.parquet`.
- **backfill**: fetching and processing historical candles ahead of time. The orchestrator serves it over HTTP on `:11090`. `src/orchestrator/cmd/orchestrator/backfill.go`.
- **candle close**: the end of a 3-hour UTC candle; the bar a playbook decides on.
- **catalog**: the store of experiments and instances with their status, metrics and period stats. Postgres (`DATABASE_URL` is required). The legacy single backtest keeps `data/backtests.sqlite`. `src/backtest/experiment_catalog.go`, `src/experimentapi/pgcatalog/`.
- **consensus (signal-consensus)**: playbook that combines several indicator sub-signals into buy, hold or sell. `src/calculation-signal/playbooks/signal-consensus/`.
- **replay consistency test**: test that runs the C++ sweep and the replay (`ReplayKPI`) on seeded lakes and checks the journals, orders, fills and period statistics against each other. `TestSmaCrossCppReplayConsistent`, `TestConsensusCppReplayConsistent` in `src/backtest/cpp_replay_test.go`.
- **experiment**: one run definition with shared settings (date range, capital, costs, provider) and many instances. Stored in the catalog; workspace `data/experiments/<experiment_id>/`.
- **fill**: a simulated order execution (side, quantity, price, costs). `src/execution/`, `src/internal/venue/sim`; per-instance fills in the instance `fills.parquet`.
- **grid**: experiment mode that expands axes of parameter values into the cartesian product of instances (up to 20000). `src/experimentapi/grid.go`.
- **instance**: one strategy configuration (playbook, strategy config, one symbol) inside an experiment. Workspace `data/experiments/<experiment_id>/instances/<instance_id>/` (`ExperimentWorkspaceDir`, `src/backtest/store/types.go`).
- **KPI window**: the indicator values per bar, loaded into memory for replay. `kpi.Window`, `src/internal/kpi/window.go`; built by `BuildKPIRange` in `src/calculation-technical-indicator/runner/range.go`.
- **KPI worker**: goroutine pool that replays C++ actions against bar closes and builds the instance report. `RunKPIWorkers`, `ReplayKPI` in `src/calculation-signal/runner/kpi_worker.go`.
- **lake**: the Hive-partitioned Parquet candle store under `data/parquet/provider=.../base=.../quote=.../year=...` (or S3/Floci). `src/shared/datalayout.go`, `src/internal/lakeobject`.
- **ma-cross**: moving-average crossover playbook. `src/calculation-signal/playbooks/ma-cross/`; C++ in `src/calculation/`.
- **orchestrator**: Go process that runs job images through Docker and hosts the Experiment API and backfill. `src/orchestrator/`.
- **playbook**: a named trading strategy (`strategy.go`, `DESCRIPTION.md`) mapped by `src/calculation-signal/internal/playbooks/registry.go`.
- **prepare / simulate / finalize**: the three phases of an experiment run. Prepare checks market data and builds indicators; simulate runs instances (in batches when `EXPERIMENT_RUN_BACKEND` is `batched` or `sfn`); finalize writes the performance from the trade outcome files, sets the final status and removes the prepare cache. `src/backtest/run_experiment_phases.go`.
- **sizing (heat, unit risk, maxUnits)**: position sizing from a stop distance. `unitRiskPct` is the share of equity lost if the stop hits, `heatPct` caps total open risk, `maxUnits` caps pyramided buys. `src/calculation-signal/internal/sizing/sizing.go`, [position-sizing](../src/calculation-signal/playbooks/position-sizing/DESCRIPTION.md).
- **shard**: a slice of instances of one symbol and playbook sent to one C++ `--sweep` process, at most `EXPERIMENT_CPP_SHARD_MAX` (default 500). `src/backtest/cpp_engine.go`.
- **sweep**: one C++ strategy run over many configurations at once (`--sweep`, protobuf `SweepRequest` in, `RunReport` out). `src/backtest/cpp_sweep.go`.
- **unavailable hours**: exchange hours confirmed missing, recorded in the `ingest_unavailable` table instead of invented candles; they do not block an experiment. `src/ingest/internal/job/run.go`, `src/backtest/coverage.go`.
- **workspace**: the per-instance directory holding its paper wallet, `report.json` and the Parquet files (fills, orders, journals, trade outcomes). See instance.
