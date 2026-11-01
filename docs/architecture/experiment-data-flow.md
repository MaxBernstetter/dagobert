# Experiment and instance data flow

How an experiment goes from a click in the browser to fills, reports and rows in the database. Settings and tolerances are in [backtesting.md](backtesting.md); the live per-candle pipeline is in [pipeline-data-flow.md](pipeline-data-flow.md).

- **Experiment**: one row with shared settings (range, capital, costs, provider) and N instances.
- **Instance**: one strategy config (playbook + `strategyConfig`) on one symbol. Its workspace is `data/experiments/<experiment_id>/instances/<instance_id>/` (`store.ExperimentWorkspaceDir`): `report.json` and the Parquet files `fills.parquet`, `orders.parquet`, `journals.parquet`, `trade_outcomes.parquet` (`src/internal/instanceparquet`).

## The picture

```mermaid
sequenceDiagram
    participant B as Browser
    participant A as Experiment API (inside orchestrator)
    participant P as Postgres catalog
    participant O as Orchestrator fan-out
    participant W as dagobert-backtest container
    participant L as Lake and workspaces
    B->>A: POST /api/v1/experiments
    A->>P: insert experiment (materializing), instances (draft)
    A->>P: status draft
    B->>A: POST /api/v1/experiments/{id}/run
    A->>P: experiment and instances running
    A->>O: hook RunExperiment (202)
    O->>W: prepare
    W->>L: coverage, ingest gaps, indicator window, KPI handoff files
    O->>W: simulate (one or many batches)
    W->>P: batched period stats and UPDATE per instance
    W->>L: report.json and Parquet files per instance
    O->>W: finalize
    W->>L: read trade_outcomes.parquet per instance
    W->>P: performance, final status, drop the prepare cache
    loop every 1500 ms
        B->>A: GET /experiments/{id}/progress
        A->>P: read row
    end
```

The simulate phase, per instance:

```mermaid
flowchart TD
    S["RunExperimentSimulate"] --> C1["BuildBarRange once per symbol"]
    C1 --> C2["shards per symbol and playbook<br/>RunSweep (C++ strategy binary --sweep)"]
    C2 --> C3["ReplayActions"]
    C3 --> C4["RunKPIWorkers (ReplayKPI, Go)"]
    C4 --> Q["result queue (instanceResult)"]
    Q --> PS["runPersister (one goroutine)"]
    PS --> R["report.json (+ S3 copy) and Parquet: fills, orders, journals, trade outcomes"]
    PS --> PG["Postgres: period stats, UPDATE instances (status, timing, metrics), progress"]
```

## Lifecycle

Statuses are the strings in `src/backtest/store/store.go` and the SQL in `db/sqlc/queries/`.

```mermaid
stateDiagram-v2
    [*] --> materializing: create
    materializing --> draft: instances inserted
    materializing --> failed: expansion error
    draft --> running: run
    running --> completed: all instances completed
    running --> partial: some failed
    running --> failed: none completed, or setup error
    running --> cancelled: cancel
    completed --> running: run again
    partial --> running: run or retryFailed
    failed --> running: run or retryFailed
    cancelled --> running: run
```

```mermaid
stateDiagram-v2
    [*] --> draft: materialize
    draft --> running: experiment run
    running --> completed: batch committed
    running --> failed: compute, symbol or persist error
    running --> cancelled: cancel
    failed --> running: retryFailed
```

Rules: `POST .../run` returns 409 while the experiment is `running`. `retryFailed` returns 400 when no instance failed, and re-simulates only `failed` instances (`EXPERIMENT_RETRY_FAILED=1` in the worker). An instance is `completed` only after its persistence batch committed.

## Stages

| Stage | Entry point | Inputs | Outputs / state | Failure and retry |
|---|---|---|---|---|
| Create | `handleCreateStudy` -> `createStudy` (`src/experimentapi/create.go`) | settings + MANUAL list or GRID axes (`specsFromGrid`, max 20 000 valid combos) | `experiment` row `materializing`, then `experiment_instance` rows, status `draft` | up to 250 instances (`syncMaterialize`) in the request, more via `materializeAsync`; error -> experiment `failed` |
| Run request | `handleRunExperiment` (`handlers.go`) | experiment id, `retryFailed` | experiment and instances `running`; hook `RunExperiment` | 409 if running; orchestrator ignores the hook call if another job runs (single-flight) |
| Fan-out | `runStudyJob` (`src/orchestrator/cmd/orchestrator/experiment_fanout.go`) | `EXPERIMENT_RUN_BACKEND` | worker containers with `EXPERIMENT_PHASE` | any non-zero exit stops the job; job state is in memory only |
| Prepare | `RunExperimentPrepare` (`src/backtest/run_experiment_phases.go`) | settings, Parquet lake, `ingest.sqlite` | progress stages 1-3; workspaces, paper wallet; KPI handoff files (`WriteAllExperimentSymKPI`, `WriteExperimentPrepareMeta`) | symbol without coverage: `ingestExperimentRange` then re-check; still failing -> `failSymbol` fails its instances; unavailable hours do not block |
| Simulate | `RunExperimentSimulate` | handoff files, instance ids | progress stage 4; instance results to the persister | per-instance error -> `failed` row; function only errors on setup |
| Persist | `runPersister` (`persist_pipeline.go`) | `instanceResult` queue | `report.json` and Parquet files per instance; Postgres period stats, status, timing, metrics | Parquet write error fails the instance (its files are removed); batch retried 3x (backoff), then bisected to single instances; last resort `failed` with `persist: ...` |
| Finalize | `RunExperimentFinalize` | instance statuses, `trade_outcomes.parquet` | `experiment_performance` (Postgres), final status, then the prepare cache (`.cache`) is removed | zero completed -> `failed`; any failed -> `partial`; else `completed`. Re-finalizing a finished experiment is not supported (the cache is gone); a new run re-prepares it |

The catalog is Postgres. The worker needs `DATABASE_URL` (`pgcatalog.Open`); without it the run fails (`experiment_catalog.go`).

Where state lives:

| State | Location |
|---|---|
| Experiment, instances, metrics, period stats, performance | Postgres (`db/sqlc/schema.sql`: `experiment`, `experiment_instance`, `experiment_instance_period_stat`, `experiment_performance`, `experiment_period_stat`, `experiment_outcome_stat`) |
| Candles | `data/parquet/provider=.../` (read-only here), gaps in `ingest.sqlite` `unavailable` |
| Indicator window per symbol | `data/experiments/<experiment_id>/.cache` (handoff files from prepare to simulate and finalize), removed at the end of finalize |
| Per instance | `data/experiments/<experiment_id>/instances/<instance_id>/`: paper wallet, `report.json`, `fills.parquet`, `orders.parquet`, `journals.parquet`, `trade_outcomes.parquet` |
| Running job (id, error, finished) | orchestrator memory (`experimentJob`) |

Production `technical_indicator.sqlite` is never written by an experiment.

## One instance in detail

The experiment engine is C++ only. The strategy decides in C++; Go replays the decisions with the same fill model as the live pipeline, so the numbers an experiment shows are the numbers the live simulation would produce for the same actions.

1. `BuildBarRange` (`src/calculation-technical-indicator/runner/bar_range.go`) once per symbol: aggregated bars, sub-candle closes, warm-up bars.
2. `cppSimulate` (`src/backtest/cpp_engine.go`) groups instances by symbol and playbook, cuts shards (see knobs), and per shard `BuildSweepRequest` then `RunSweep` runs `strategy-sma-cross` or `strategy-signal-consensus` with `--sweep` (protobuf `SweepRequest` in, run report out, `src/backtest/cpp_sweep.go`).
3. `ReplayActions` turns the reported sized actions into `ReplayAction`s. `RunKPIWorkers` -> `ReplayKPI` (`src/calculation-signal/runner/kpi_worker.go`) replays them against the bar closes with `execrunner.SimulateBarWithModel` (`src/internal/venue/sim`: spread, slippage and fee in bps against the bar close), rescales sizes by the replayed wallet equity over the C++ book equity, and builds the report with `BuildInstanceReportFromSim`. The orders, journals and fills it returns are the instance files. No disk or database access.
4. The same queue and persister as before. A run uses `VENUE_SIMULATE=1`; sizing comes from `allocate.PolicyFromEnv()` (`ALLOC_*`, copied from the experiment settings by `applyExperimentEnv`).

No fallback: an unsupported playbook or a missing binary fails the instance with the variable name; a crashed shard fails all its instances.

### Checks

`src/backtest/cpp_replay_test.go` runs the sweep and the replay over seeded lakes and checks the persisted shape: one journal and one fill order per fill, the same values in both, and period statistics that add up to the trade outcomes. The configs must fire vol_cut and pyramid (ma-cross) and close trades. Details in [backtesting.md](backtesting.md).

## Progress and what the frontend polls

Base URL `VITE_EXPERIMENT_API_URL` (dev proxy in `src/frontend/vite.config.ts`). Routes: `src/experimentapi/api.go`.

| Endpoint | Shows | Used by |
|---|---|---|
| `GET /api/v1/experiments` | experiment list | list view |
| `GET /api/v1/experiments/{id}` | summary only | detail header |
| `GET .../{id}/progress` | status, stage (1 prepare, 2 market data, 3 indicators, 4 instance simulation), instance done/total, failed count | polled every 1500 ms while selected (`components/experiments-panel.tsx`), then the instance page reloads |
| `GET .../{id}/instances` | paginated, SQL sort and filter on metric columns | instance table |
| `GET .../instances/{instanceId}` | one instance row | detail |
| `.../report`, `.../kpis`, `.../activity`, `.../chart-series?from=&to=[&maxPoints=N]` (unix s) | report, win-rate ladders, fills, chart data | instance detail |

Every response is gzip-coded when the client asks for it (`Accept-Encoding: gzip`). `maxPoints` on `chart-series` buckets the candles to at most N (OHLC aggregated, indicators at the bucket's last candle); portfolio and markers stay exact. The frontend sends about two candles per viewport pixel (400 to 4000), and each zoom or range preset fetches its window with the same budget. See `src/experimentapi/README.md`.
| `GET .../{id}/performance` | experiment-level reduction | performance view |
| `GET .../{id}/status` | aggregate status (`status.go`) | not used by `api.ts` |
| `POST .../{id}/run`, `POST .../{id}/cancel`, `DELETE .../{id}` | actions | buttons |

## Knobs

| Variable | Default | Meaning |
|---|---|---|
| `DATABASE_URL` | unset | Postgres catalog, required by the worker (a run fails without it) and by the Experiment API (routes are mounted only when set) |
| `EXPERIMENT_SIM_WORKERS` | CPU count | Go KPI replay workers (CPU bound) |
| `EXPERIMENT_CPP_PROCESSES` | CPU count | concurrent C++ `--sweep` processes |
| `EXPERIMENT_CPP_SHARD_MAX` | 500 | upper bound of instances per shard (shard = `ceil(n/processes)` capped here) |
| `CPP_SMA_CROSS_BIN`, `CPP_SIGNAL_CONSENSUS_BIN` | set in the worker image (`/usr/local/bin/...`) | C++ strategy executables |
| `EXPERIMENT_RUN_BACKEND` | `docker` | `docker` = one container, all phases; `batched` = prepare, parallel simulate batches, finalize; `sfn` = same with Step Functions Map |
| `EXPERIMENT_BATCH_SIZE` | 64 | instances per simulate container (batched/sfn) |
| `EXPERIMENT_MAP_MAX_CONCURRENCY` | 8 | parallel simulate batches in the local pool |
| `EXPERIMENT_SFN_ARN` | unset | state machine for the Map fan-out; unset uses the local pool |
| `EXPERIMENT_ARTIFACTS` | `fs` | `s3` also copies `report.json` to S3 |
| `EXPERIMENT_PHASE`, `EXPERIMENT_INSTANCE_IDS`, `EXPERIMENT_INSTANCE_FILE`, `EXPERIMENT_RETRY_FAILED` | set by the orchestrator | worker internals |

Persistence constants (not env): batch 200 results or 250 ms, queue `min(instances, 4096)`, report writers 8, 4 concurrent commits on Postgres, 3 retries (`persist_pipeline.go`).

## Known limits

- Experiments completed before the Parquet instance files (#164) have no `trade_outcomes.parquet`, and `fills`/`orders`/`journals` are missing too. Their activity and charts are empty and their trade outcomes are gone from `/kpis` and `/performance`. Re-running the experiment rebuilds them; re-finalizing without re-running does not (the prepare cache is removed at finalize). This is a deliberate behaviour change; no backfill exists.
- Fills are not written to Postgres any more, so large runs no longer pay for a fill COPY. The Parquet files of a 3000-instance run take 232 MiB (zigzag lake) and 484 MiB (random walk) and the run takes 14.9 s and 16.9 s (medians of three); see `docs/development/05-cpp-benchmark.md`.
- Single-flight: the orchestrator runs one backtest or experiment job at a time; the run hook silently returns if one is active, and job status is lost on orchestrator restart.
- The legacy single-backtest path (`backtest.Run`, `BACKTEST_ID`, `POST /backtest/run`, `task backtest-run`) is still wired and untouched by this flow.
- The C++ engine supports only `ma-cross` and `signal-consensus`.
