# Backtesting

Backtests are catalogued in `data/backtests.sqlite` with per-run workspaces under `data/backtests/<id>/`. Production `technical_indicator.sqlite` is never written during a backtest; indicator bars live in `backtest_kpi` keyed by `backtest_id`.

## Runtime

```text
POST /api/backtests/:id/run  →  orchestrator POST /backtest/run
  →  docker run trading-copilot-backtest (BACKTEST_ID, DATA_DIR=/data)
       →  optional ingest (shared parquet)
       →  batch TI into backtest_kpi
       →  FastSimulatedPipeline (workspace sqlite + paper wallet)
```

The Strategy page reads live `calculation_signal.sqlite` / `execution.sqlite` only. Reports use backtest-scoped APIs under `/api/backtests/[id]/…`.

While `status = running`, `backtests.sqlite` carries staged progress: `progress_stage` / `progress_stages` (four phases: prepare, market data, indicators, strategy simulation), `progress_stage_label`, and bar counters during simulation. The UI derives an overall percentage from those fields.

Execution simulation uses `src/internal/venue/sim` (spread, slippage, and fee in bps vs bar close) and `allocate.PolicyFromEnv()` for sizing (`ALLOC_*`). Values are copied from the backtest settings JSON into the job environment at run start.

## Entrypoints

- Job image: `trading-copilot-backtest` (`src/cmd/backtest`)
- Local: `task backtest-run BACKTEST_ID=<uuid>` (after `task build:jobs`)
