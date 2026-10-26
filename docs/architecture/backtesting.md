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
            (shared Window; no signal/action SQLite; fills+wallet only; report built in memory)
       →  persistence stage drains the results in batches (see below)
       →  UpsertExperimentKPIRows once + mark completed
```

Instance artifact: `data/experiments/<experiment_id>/instances/<instance_id>/report.json` (runtime metrics, StrategyKPIs, nested monthly/yearly return stats + series). Catalog keeps `started_at` / `finished_at` plus **list-projection metric columns** (`metric_sharpe`, `metric_cagr`, …) for the instance table — full report is loaded on demand via `GET .../instances/{id}/report`. Win rate, its month and year rates, and the gain and loss ladder are stored in Postgres (`experiment_instance` counts, period win counts, per-sell outcomes, and the experiment reduction) and read by `GET .../kpis` and `GET .../performance`.


### Simulation stages and persistence

The simulate phase is a three-stage pipeline. Database writes never run on the calculation path.

1. **Compute** (`EXPERIMENT_SIM_WORKERS` goroutines): run the strategy, read the instance fills once from the workspace journal, and build the report (KPIs, statistics, period series) in memory. The result (`instanceResult`: instance, report, fills, timing, error) goes to a queue. Compute workers touch no database and no mutex.
2. **Queue**: a bounded channel with capacity `min(instances, 4096)` (about 10 KB per result). The send only blocks when more than 4096 results wait for persistence.
3. **Persistence** (one goroutine, `persist_pipeline.go`): collects up to 200 results or 250 ms, whichever comes first, writes `report.json` (and the S3 copy) with a pool of 8, then commits the batch in **one transaction**:
   - Postgres (`pgcatalog.PersistInstanceBatch`): `DELETE ... WHERE instance_id = ANY($1)` on fills, trade outcomes and period stats, `COPY` for the three tables, one `UPDATE experiment_instance ... FROM unnest(...)` setting status, timing and metric columns together, and the progress counter.
   - SQLite (`store.PersistInstanceBatch`): the same status, timing and metric updates plus progress in one transaction.

Compute and KPI errors reach the database only as `failed` rows in these batches.

**Semantics.** An instance is `completed` only after its batch committed. A failed batch is retried 3 times (backoff 100 ms, 400 ms, 1.6 s), then split in half recursively down to single instances. An instance that still fails is marked `failed` with an error starting `persist:` (and stays `running` if even that write fails); both are picked up by `EXPERIMENT_RETRY_FAILED`. `RunExperimentSimulate` closes the queue when compute finishes and returns only after the persister has drained it, so `RunExperimentFinalize` counts final statuses. Test seams: `PersistCommitHook`, `PersistRetryBackoff`, `SimulateComputeDoneHook` (see `tests/system/experiment/persist_test.go`).

**Measurements** (3000 `ma-cross` instances, 2 years of 1h candles, 22 workers, `tests/system/experiment` `TestBenchPersistGrid`, compose Postgres 16):

| Catalog | Instances | Before (main) | After (this change) |
|---|---|---|---|
| SQLite | 3000 | 9 min 4 s | 1 min 44 s |
| Postgres | 100 | 5 min 8 s | 7.7 s |
| Postgres | 3000 | not run to completion (about 3 s per instance, roughly 2.6 h extrapolated; a contended partial run completed 132 of 3000 in 25 min) | 2 min 39 s |

Single runs on a shared 22-core host; load average was 2 to 11 at the start of the runs, so treat the numbers as indicative. The old Postgres path issued one autocommitted statement per fill (about 1000 fills per instance), each waiting for a WAL flush, while holding a global mutex.

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
