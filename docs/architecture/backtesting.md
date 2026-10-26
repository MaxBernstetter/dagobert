# Backtesting

The UI focuses on the **Experiment Suite** (`/experiments`). Single-backtest list APIs and the Portfolio / Strategy / Data / Processing pages were removed from the frontend.

Without `DATABASE_URL` the catalog is `data/backtests.sqlite` (experiments + optional legacy `backtest` rows); with it, Postgres (also stores fills, see the data-flow doc). Instance workspaces live under `data/experiments/<id>/…`. Production `technical_indicator.sqlite` is never written during a experiment run; indicator bars for experiments live in `experiment_kpi`.

## Experiments (multi-instance)

An **Experiment** (catalogued as a experiment row) shares range/capital across N **instances** (playbook `ma-cross` or `signal-consensus` with its `strategyConfig`, plus per-instance `symbol`). KPI rows live in `experiment_kpi` keyed by `(experiment_id, symbol, …)` — one shared window per symbol. Instance workspaces are under `data/experiments/<experiment_id>/instances/<instance_id>/`.

Create UI supports **Manual** instances or **GRID** search: options render as cards (≥2 per row). Enum axes are multi-select; number axes use a dual-thumb min/max range slider plus a tick field. The client posts **settings + grid axes** (or a small manual instance list); the server expands the cartesian product (invalid configs dropped, max 20 000 valid) asynchronously (`status: materializing` → `draft`). Experiment GET is a **summary only**; instances are a **paginated** child resource with SQL sort/filter on denormalized KPI columns written at sim complete. See `docs/grooming/experiment-api-scale.md`.

**Retry failed:** `POST /api/v1/experiments/{id}/run` with `{ "retryFailed": true }` sets `EXPERIMENT_RETRY_FAILED=1` so only previously failed instances are re-simulated; completed instances and their workspaces are left untouched.

Run flow (sequence, phases, stages, state machines, knobs): [experiment-data-flow.md](experiment-data-flow.md). In short: the Experiment API is mounted inside the orchestrator (needs `DATABASE_URL`); `POST /api/v1/experiments/{id}/run` calls the `RunExperiment` hook, which starts `runStudyJob`. That runs the `dagobert-backtest` image as one container (`EXPERIMENT_PHASE=all`, default backend `docker`) or as three phases `prepare`, `simulate` (in batches of `EXPERIMENT_BATCH_SIZE`, local pool or Step Functions Map) and `finalize` (`EXPERIMENT_RUN_BACKEND=batched|sfn`).

Instance artifact: `data/experiments/<experiment_id>/instances/<instance_id>/report.json` (runtime metrics, StrategyKPIs, nested monthly/yearly return stats + series). Catalog keeps `started_at` / `finished_at` plus **list-projection metric columns** (`metric_sharpe`, `metric_cagr`, …) for the instance table — full report is loaded on demand via `GET .../instances/{id}/report`. Win rate, its month and year rates, and the gain and loss ladder are stored in Postgres (`experiment_instance` counts, period win counts, per-sell outcomes, and the experiment reduction) and read by `GET .../kpis` and `GET .../performance`.


### Simulation stages and persistence

The simulate phase is a three-stage pipeline. Database writes never run on the calculation path.

1. **Compute** (`EXPERIMENT_SIM_WORKERS` goroutines): run the strategy, read the instance fills once from the workspace journal, and build the report (KPIs, statistics, period series) in memory. The result (`instanceResult`: instance, report, fills, timing, error) goes to a queue. Compute workers touch no database and no mutex.
2. **Queue**: a bounded channel with capacity `min(instances, 4096)` (about 10 KB per result). The send only blocks when more than 4096 results wait for persistence.
3. **Persistence** (one goroutine, `persist_pipeline.go`): collects up to 200 results or 250 ms, whichever comes first, writes `report.json` (and the S3 copy) with a pool of 8, then commits the batch in **one transaction**:
   - Postgres (`pgcatalog.PersistInstanceBatch`): `DELETE ... WHERE instance_id = ANY($1)` on fills, trade outcomes and period stats, `COPY` for the three tables, one `UPDATE experiment_instance ... FROM unnest(...)` setting status, timing and metric columns together, and the progress counter. Up to 4 batches commit concurrently on Postgres (separate connections; progress uses `GREATEST`).
   - SQLite (`store.PersistInstanceBatch`): the same status, timing and metric updates plus progress in one transaction.

Compute and KPI errors reach the database only as `failed` rows in these batches.

**Semantics.** An instance is `completed` only after its batch committed. A failed batch is retried 3 times (backoff 100 ms, 400 ms, 1.6 s), then split in half recursively down to single instances. An instance that still fails is marked `failed` with an error starting `persist:` (and stays `running` if even that write fails); both are picked up by `EXPERIMENT_RETRY_FAILED`. `RunExperimentSimulate` closes the queue when compute finishes and returns only after the persister has drained it, so `RunExperimentFinalize` counts final statuses. Test seams: `PersistCommitHook`, `PersistRetryBackoff`, `SimulateComputeDoneHook` (see `tests/system/experiment/persist_test.go`).

**Measurements** (3000 `ma-cross` instances, 2 years of 1h candles, 22 workers, `tests/system/experiment` `TestBenchPersistGrid`, compose Postgres 16):

| Catalog | Instances | Before (main) | After the persistence pipeline |
|---|---|---|---|
| SQLite | 3000 | 9 min 4 s | 1 min 44 s (Go engine; 253 s measured later, see `docs/development/05-cpp-benchmark.md`) |
| Postgres | 100 | 5 min 8 s | 7.7 s |
| Postgres | 3000 | not run to completion (about 3 s per instance, roughly 2.6 h extrapolated; a contended partial run completed 132 of 3000 in 25 min) | 2 min 39 s (Go engine) |

With `EXPERIMENT_ENGINE=cpp` the same 3000 instances take 4 to 6 s on SQLite and about 100 s on Postgres (195 s before #162, 229 s on the random-walk lake), see `docs/development/05-cpp-benchmark.md`; the Postgres time is the COPY of about 5 M fills (foreign-key checks), not compute. Single runs on a shared 22-core host; load average was 2 to 11 at the start of the runs, so treat the numbers as indicative. The old Postgres path issued one autocommitted statement per fill (about 1000 fills per instance), each waiting for a WAL flush, while holding a global mutex.

### KPI worker (actions to KPIs)

For the C++ path (#130) the strategy outcome is a list of sized actions. `src/calculation-signal/runner/kpi_worker.go` turns them into fills and the instance report without disk or database access: `ReplayKPI` replays the actions against the bar closes with the same fill model as simulated execution (`execrunner.SimulateBarWithModel`, quantities read back through the journal strings like the SQLite path) and builds the report with `BuildInstanceReportFromSim`. `RunKPIWorkers` runs a goroutine pool over batches of `KPIJob` and emits one `[]KPIResult` batch per input batch, ready for the persistence stage. `RunExperimentSimulate` uses it when `EXPERIMENT_ENGINE=cpp` (see below).


### Sparse lake (exchange holes)

Ingest does **not** synthesize candles. When Binance returns no closed kline for a requested hour, that open time is recorded in `ingest.sqlite` table `unavailable` (detail **32** if nothing was written; detail **30** is transport/retry failure only). Coverage `Ready` allows unavailable hours; only **retryable** missing hours block a experiment/backtest.

TI aggregation already omits incomplete 3h buckets when any source 1h is missing, so the sim timeline skips those bars. Strategies only `Evaluate` bars that exist; position/state persists across gaps unless the strategy acts. `framework.Context` exposes `GapFromPrevious` / `PreviousBarOpen` for strategies that care (ma-cross ignores them and holds implicitly).

Signal/fill playbook keys use `ma-cross.yaml#<config_hash>` so variants never collide.

## CLI single backtest (optional)

Legacy: `task backtest-run BACKTEST_ID=…` still runs the job image with `BACKTEST_ID` (orchestrator `POST /backtest/run`). There is no Backtesting page in the UI and the experiment flow does not use it.

Reports use experiment instance APIs under `/api/v1/experiments/{id}/instances/{instanceId}/…`.

While `status = running`, the catalog carries staged progress: `progress_stage` / `progress_stages` (prepare, market data, indicators, instance simulation), `progress_stage_label`, and instance counters.

Execution simulation uses `src/internal/venue/sim` (spread, slippage, and fee in bps vs bar close) and `allocate.PolicyFromEnv()` for sizing (`ALLOC_*`). Values are copied from the backtest/experiment settings JSON into the job environment at run start.

## What a run is

A backtest replays a playbook over historical 3-hour bars and reports fills, wallet and KPIs, compared with buy-and-hold over the same range. It runs the same playbook code as the live pipeline. Results are simulated and are not a claim of profitable trading.

Retry, progress and outputs are described above. The Performance view compares total return with buy-and-hold from the first bar, starting from the same USDC.

## Running it

1. Start the stack (`task up`) and build the job images once (`task build:jobs`, includes `dagobert-backtest`).
2. Open **Experiments** in the UI, create an experiment, then **Run experiment**. Or from the CLI: `task experiment-run EXPERIMENT_ID=<uuid>`.

After backend changes, rebuild the orchestrator (`docker compose build orchestrator && docker compose up -d orchestrator`) so job containers start with the right Docker flags (the job writes `data/backtests.sqlite`). The playbook registry is compiled into the job image, so after adding or changing a playbook run `docker build -f docker/backtest/Dockerfile -t dagobert-backtest .`.

**Market data.** Every run fills gaps in `data/parquet` for the chosen range (missing hours only). **Force re-download** replaces existing candles in that range; a forced full-year refresh can sit on the market-data stage for a long time. Target runtime is about one minute for one year of 3-hour bars when Parquet is warm. For a demo history use `task seed-demo-year` ([seed-demo-year](../development/seed-demo-year.md)).

## Simulation settings

Stored with each experiment and copied into the job environment when the run starts (defaults in `src/backtest/store/store.go`).

| Setting | Env in the job | Default |
|---|---|---|
| Starting book | `paper_venue.json` venue quote | 100 000 USDC |
| Target weight | `ALLOC_TARGET_WEIGHT_PCT` | 100 % |
| Cash reserve | `ALLOC_CASH_RESERVE_QUOTE` | 0 |
| Minimum order | `ALLOC_MIN_ORDER_QUOTE` | 1 |
| Spread | `SIM_SPREAD_BPS` | 4 bps (full width) |
| Slippage | `SIM_SLIPPAGE_BPS` | 2 bps |
| Fee | `SIM_FEE_BPS` | 6 bps |

The starting book is initial USDC plus initial USDT, placed entirely in the venue quote (USDC for BTC and ETH). A buy may spend that whole balance, and a sell exits the whole base position. A run already stored with a lower target weight keeps that setting until it is edited and run again.

The reference price is the close of the 3-hour bar. A buy fills at `close x (1 + spread/2 + slippage)`, a sell at `close x (1 - spread/2 - slippage)`, and the fee applies to the quote notional after that. Live simulated execution (`VENUE_SIMULATE=1`) uses the same `SIM_*` variables. The model uses fixed basis-point values, not the order book.

## Entrypoints

- Job image: `dagobert-backtest` (`src/cmd/backtest`) — `BACKTEST_ID` or `EXPERIMENT_ID`
- Local: `task backtest-run BACKTEST_ID=<uuid>` or `task experiment-run EXPERIMENT_ID=<uuid>` (after `task build:jobs`)
- Experiment sim concurrency: `EXPERIMENT_SIM_WORKERS` (default 4; set in `.env` / compose for the orchestrator)

### C++ sweep client (CALC.30 S4a)

`src/backtest/cpp_sweep.go` builds a `SweepRequest` (bars from `tirunner.BuildBarRange`: aggregated bars with the sub-candle closes, warmup bars before the first evaluated bar), runs the playbook executable with `--sweep` and converts the reported actions into `signalrunner.ReplayAction`. Sized actions carry the C++ book equity; `ReplayKPI` rescales `quote_notional`/`base_qty` by the replayed wallet equity (marked at the bar close) over that equity and sizes with the market-action policy. Only `ma-cross` and `signal-consensus` are supported; other playbooks fail. `signal-consensus` sends the full resolved config; the cooldowns `minBarsFlatAfterSell`/`minBarsLongBeforeSell` are proto `optional` fields, always set from the resolved config, so an explicit 0 means no cooldown on both engines while an unset field keeps the default 8/4 (#128); the equivalence test covers zero cooldowns. The Go path stays the default (engine switch below). The Go experiment path cannot trade on its first evaluated bar, so the sweep's `trade_from_ms` is the bar after it. Both playbooks are verified against the Go path, sized configs included (vol_cut and pyramid fire): `TestSmaCrossGoVsCppEquivalence` and `TestConsensusGoVsCppEquivalence` (fill timestamp and side exact, qty and price within 1e-9, KPIs within 1e-6). The consensus test runs a 1h-candle lake and the same lake with missing hours; the `BuildBarRange` warm-up is enough for its longest series (SMA48, MACD, ATR). A 3h-candle lake is not a useful consensus input: `rsi-3h` is never ready on it, so no trade happens on either path. Notes on consensus windows (RSI from the 1h closes inside the bar, volume ratio always 1, MACD 4/8/3 seeded on the last N values) are in the playbook `DESCRIPTION.md` and `src/calculation-signal/playbooks/position-sizing/DESCRIPTION.md`.

### Engine switch (CALC.30 S4b)

`EXPERIMENT_ENGINE=go|cpp` (default `go`, anything else fails the simulate stage) selects the compute stage of `RunExperimentSimulate` for the whole run. `go` is unchanged. `cpp` (`src/backtest/cpp_engine.go`) runs C++ sweep, then `ReplayActions`, `RunKPIWorkers` and the same `runPersister` stage:

- Instances are grouped by symbol and playbook, each group is cut into shards of `ceil(n/processes)` instances capped at `EXPERIMENT_CPP_SHARD_MAX` (default 500); `EXPERIMENT_CPP_PROCESSES` (default: CPU count) sets the number of concurrent `--sweep` processes. Bars come from `BuildBarRange` once per symbol.
- The executable of a playbook comes from an env var; `ma-cross` uses `CPP_SMA_CROSS_BIN`, `signal-consensus` uses `CPP_SIGNAL_CONSENSUS_BIN` (built by `task cpp:build PRESET=optimized`; baked into the `dagobert-backtest` image, see below). Missing or non-executable binaries fail the instances.
- There is no per-instance fallback to Go. A crashed or malformed shard marks all its instances failed with the process error. A playbook the C++ path does not support (everything except `ma-cross` and `signal-consensus`) marks its instances failed with `C++ engine does not support playbook ...`; if no instance succeeds the experiment fails in finalize, otherwise it is `partial`.
- System tests: `tests/system/experiment/engine_test.go`.

Worker image: `docker/backtest/Dockerfile` builds `strategy-sma-cross` and `strategy-signal-consensus` in a `debian:trixie` stage with `DAGOBERT_PORTABLE=ON` (baseline CPU code, no `-march=native`), copies them to `/usr/local/bin` of the final `distroless/cc` image and defaults `CPP_SMA_CROSS_BIN=/usr/local/bin/strategy-sma-cross` and `CPP_SIGNAL_CONSENSUS_BIN=/usr/local/bin/strategy-signal-consensus`. The dependency prefix (Arrow, protobuf, ...) is a separate `cpp-deps` stage that only sees the recipe files, so BuildKit reuses it until `deps/` changes.
