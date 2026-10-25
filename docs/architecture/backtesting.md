# Backtesting

The UI focuses on the **Experiment Suite** (`/experiments`). Single-backtest list APIs and the Portfolio / Strategy / Data / Processing pages were removed from the frontend.

Catalog DB remains `data/backtests.sqlite` (experiments + optional legacy `backtest` rows). Instance workspaces live under `data/experiments/<id>/…`. Production `technical_indicator.sqlite` is never written during a experiment run; indicator bars for experiments live in `experiment_kpi`.

## Experiments (multi-instance)

An **Experiment** (catalogued as a experiment row) shares range/capital across N **instances** (ma-cross `strategyConfig`: `fastHr` / `slowHr`, plus per-instance `symbol`). KPI rows live in `experiment_kpi` keyed by `(experiment_id, symbol, …)` — one shared window per symbol. Instance workspaces are under `data/experiments/<experiment_id>/instances/<instance_id>/`.

Create UI supports **Manual** instances or **GRID** search: options render as cards (≥2 per row). Enum axes are multi-select; number axes use a dual-thumb min/max range slider plus a tick field. The client posts **settings + grid axes** (or a small manual instance list); the server expands the cartesian product (invalid configs dropped, max 20 000 valid) asynchronously (`status: materializing` → `draft`). Experiment GET is a **summary only**; instances are a **paginated** child resource with SQL sort/filter on denormalized KPI columns written at sim complete. See `docs/grooming/experiment-api-scale.md`.

**Retry failed:** `POST /api/v1/experiments/:id/run` with `{ "retryFailed": true }` (and orchestrator `retryFailed`) sets `EXPERIMENT_RETRY_FAILED=1` so only previously failed instances are re-simulated; completed instances and their workspaces are left untouched.

```text
POST /api/v1/experiments/:id/run  →  orchestrator POST /experiment/run
  →  docker run dagobert-backtest (EXPERIMENT_ID, DATA_DIR=/data, EXPERIMENT_SIM_WORKERS)
       →  CheckRawCoverage → chunked ingest for retryable gaps → re-check
       →  BuildKPIRange once (warm TI parquet cache when present) → in-memory kpi.Window
       →  worker pool (EXPERIMENT_SIM_WORKERS, default 4) slim FastSimulatedPipelineOpts per instance
            (shared Window; no signal/action SQLite; fills+wallet only; report.json with run+KPIs+period series)
       →  UpsertExperimentKPIRows once + mark completed
```

Instance artifact: `data/experiments/<experiment_id>/instances/<instance_id>/report.json` (runtime metrics, StrategyKPIs, nested monthly/yearly return stats + series). Catalog keeps `started_at` / `finished_at` plus **list-projection metric columns** (`metric_sharpe`, `metric_cagr`, …) for the instance table — full report is loaded on demand via `GET .../instances/{id}/report`. Win rate, its month and year rates, and the gain and loss ladder are stored in Postgres (`experiment_instance` counts, period win counts, per-sell outcomes, and the experiment reduction) and read by `GET .../kpis` and `GET .../performance`.


### Sparse lake (exchange holes)

Ingest does **not** synthesize candles. When Binance returns no closed kline for a requested hour, that open time is recorded in `ingest.sqlite` table `unavailable` (detail **32** if nothing was written; detail **30** is transport/retry failure only). Coverage `Ready` allows unavailable hours; only **retryable** missing hours block a experiment/backtest.

TI aggregation already omits incomplete 3h buckets when any source 1h is missing, so the sim timeline skips those bars. Strategies only `Evaluate` bars that exist; position/state persists across gaps unless the strategy acts. `framework.Context` exposes `GapFromPrevious` / `PreviousBarOpen` for strategies that care (ma-cross ignores them and holds implicitly).

Signal/fill playbook keys use `ma-cross.yaml#<config_hash>` so variants never collide.

## CLI single backtest (optional)

`task backtest-run BACKTEST_ID=…` still runs the job image with `BACKTEST_ID` (orchestrator `POST /backtest/run`). There is no dedicated Backtesting page in the UI.

Reports use experiment instance APIs under `/api/v1/experiments/[id]/instances/[instanceId]/…`.

While `status = running`, the catalog carries staged progress: `progress_stage` / `progress_stages` (prepare, market data, indicators, instance simulation), `progress_stage_label`, and instance counters.

Execution simulation uses `src/internal/venue/sim` (spread, slippage, and fee in bps vs bar close) and `allocate.PolicyFromEnv()` for sizing (`ALLOC_*`). Values are copied from the backtest/experiment settings JSON into the job environment at run start.

## Entrypoints

- Job image: `dagobert-backtest` (`src/cmd/backtest`) — `BACKTEST_ID` or `EXPERIMENT_ID`
- Local: `task backtest-run BACKTEST_ID=<uuid>` or `task experiment-run EXPERIMENT_ID=<uuid>` (after `task build:jobs`)
- Experiment sim concurrency: `EXPERIMENT_SIM_WORKERS` (default 4; set in `.env` / compose for the orchestrator)
