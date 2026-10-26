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

### KPI worker (actions to KPIs)

For the C++ path (#130) the strategy outcome is a list of sized actions. `src/calculation-signal/runner/kpi_worker.go` turns them into fills and the instance report without disk or database access: `ReplayKPI` replays the actions against the bar closes with the same fill model as simulated execution (`execrunner.SimulateBarWithModel`, quantities read back through the journal strings like the SQLite path) and builds the report with `BuildInstanceReportFromSim`. `RunKPIWorkers` runs a goroutine pool over batches of `KPIJob` and emits one `[]KPIResult` batch per input batch, ready for the persistence stage. `RunExperimentSimulate` uses it when `EXPERIMENT_ENGINE=cpp` (see below).

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

### C++ sweep client (CALC.30 S4a)

`src/backtest/cpp_sweep.go` builds a `SweepRequest` (bars from `tirunner.BuildBarRange`: aggregated bars with the sub-candle closes, warmup bars before the first evaluated bar), runs `strategy-sma-cross --sweep` and converts the reported actions into `signalrunner.ReplayAction`. Sized actions carry the C++ book equity; `ReplayKPI` rescales `quote_notional`/`base_qty` by the replayed wallet equity (marked at the bar close) over that equity and sizes with the market-action policy. Only `ma-cross` is supported; other playbooks fail. The Go path stays the default (engine switch below). The Go experiment path cannot trade on its first evaluated bar, so the sweep's `trade_from_ms` is the bar after it. Known gap: sized configs differ slightly because the Go ATR is a simple mean over a 16-bar window while the C++ ATR is Wilder-smoothed over all bars. Unsized configs match the Go fills exactly (`TestSmaCrossGoVsCppEquivalence`).

### Engine switch (CALC.30 S4b)

`EXPERIMENT_ENGINE=go|cpp` (default `go`, anything else fails the simulate stage) selects the compute stage of `RunExperimentSimulate` for the whole run. `go` is unchanged. `cpp` (`src/backtest/cpp_engine.go`) runs C++ sweep, then `ReplayActions`, `RunKPIWorkers` and the same `runPersister` stage:

- Instances are grouped by symbol and playbook, each group is cut into shards of `ceil(n/processes)` instances capped at `EXPERIMENT_CPP_SHARD_MAX` (default 500); `EXPERIMENT_CPP_PROCESSES` (default: CPU count) sets the number of concurrent `--sweep` processes. Bars come from `BuildBarRange` once per symbol.
- The executable of a playbook comes from an env var; `ma-cross` uses `CPP_SMA_CROSS_BIN` (built by `task cpp:build PRESET=optimized`; it is not yet baked into the `dagobert-backtest` image, see the open item below). Missing or non-executable binaries fail the instances.
- There is no per-instance fallback to Go. A crashed or malformed shard marks all its instances failed with the process error. A playbook the C++ path does not support (everything except `ma-cross`) marks its instances failed with `C++ engine does not support playbook ...`; if no instance succeeds the experiment fails in finalize, otherwise it is `partial`.
- System tests: `tests/system/experiment/engine_test.go`.

Open item: the worker image does not contain the C++ executables yet. A first attempt (a `debian:trixie` build stage running `deps/build.sh optimized`) failed compiling Arrow's bundled snappy with clang 19 (`__m256i` unknown), and the optimized preset uses `-march=native`, which is wrong for an image built elsewhere. Until that is solved, `EXPERIMENT_ENGINE=cpp` needs `CPP_SMA_CROSS_BIN` to point at a binary mounted into the container.
