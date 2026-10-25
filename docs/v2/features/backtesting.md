# Backtesting

Status: implemented. Experiments are the supported entry point.

## What it does

A backtest replays a playbook over historical 3-hour bars and reports what it would have done: fills, wallet, and KPIs, compared with buy-and-hold over the same range. It runs the same playbook code as the live pipeline, so a backtest result is the production logic applied to past data.

An **experiment** is one run over a shared range and starting capital with N **instances**. Each instance is one parameter set of the same playbook, for example `ma-cross` with a given `fastHr` and `slowHr`, on one symbol. Instances are created manually or from a grid of parameter axes, which the server expands into a cartesian product (invalid combinations are dropped, at most 20 000 valid instances).

## Run flow

```text
POST /api/v1/experiments/:id/run  →  orchestrator POST /experiment/run
  →  docker run dagobert-backtest (EXPERIMENT_ID, DATA_DIR, EXPERIMENT_SIM_WORKERS)
       1. Check Parquet coverage for the range; ingest missing hours (retryable gaps only)
       2. Build the KPI series once per symbol into memory
       3. Simulate each instance in a worker pool (default 4 workers)
       4. Write experiment_kpi rows and mark the experiment completed
```

- **Shared window.** KPIs are computed once per symbol and shared by every instance, so the cost of a run does not grow with the number of instances.
- **Slim simulation.** Each instance records fills and wallet only. It writes no signal or action rows.
- **Retry failed.** `POST /api/v1/experiments/:id/run` with `{ "retryFailed": true }` (or `EXPERIMENT_RETRY_FAILED=1`) re-simulates only the instances that failed. Completed instances keep their workspaces.
- **Progress.** While running, the experiment reports its stage (prepare, market data, indicators, instance simulation) and instance counters.

## Outputs

- Catalogue: `data/backtests.sqlite`, with the experiment, its instances and their list metrics (Sharpe, CAGR and so on).
- Per instance: `data/experiments/<experiment_id>/instances/<instance_id>/report.json`, with runtime metrics, strategy KPIs, and monthly and yearly return statistics with their series.
- The production `technical_indicator.sqlite` is never written by a backtest.

The UI shows the experiment list, the instance table and the per-instance report at `/experiments`. The Performance view compares total return with buy-and-hold from the first bar, starting from the same USDC.

## Market data gaps

Ingest does not invent candles. When Binance has no closed candle for an hour, that hour is recorded as unavailable (see [ingestion](ingestion.md)). Coverage counts unavailable hours as ready and only retryable missing hours block a run.

The 3-hour aggregation omits any bucket with a missing 1-hour source, so the simulation skips that bar. Position state carries across the gap. Playbooks can read `GapFromPrevious` and `PreviousBarOpen` from the context; `ma-cross` ignores them.

## Simulation settings

Settings are stored with each experiment and copied into the job environment when the run starts.

| Setting | Env in the job | Default |
|---|---|---|
| Starting book | `paper_venue.json` venue quote | 100 000 USDC |
| Target weight | `ALLOC_TARGET_WEIGHT_PCT` | 100 % |
| Cash reserve | `ALLOC_CASH_RESERVE_QUOTE` | 0 |
| Minimum order | `ALLOC_MIN_ORDER_QUOTE` | 1 |
| Spread | `SIM_SPREAD_BPS` | 4 bps (full width) |
| Slippage | `SIM_SLIPPAGE_BPS` | 2 bps |
| Fee | `SIM_FEE_BPS` | 6 bps |

The starting book is initial USDC plus initial USDT, placed entirely in the venue quote (USDC for BTC and ETH). A buy may spend that whole balance, and a sell exits the whole base position. Charts and the buy-and-hold note replay equity from that same book. A run already stored with a lower target weight keeps that setting until it is edited and run again.

The reference price is the close of the 3-hour bar. A buy fills at `close × (1 + spread/2 + slippage)`, a sell at `close × (1 − spread/2 − slippage)`, and the fee applies to the quote notional after that. Live simulated execution (`VENUE_SIMULATE=1`) uses the same `SIM_*` variables.

## Running it

```bash
task build:jobs                                   # includes dagobert-backtest
task experiment-run EXPERIMENT_ID=<uuid>          # from the CLI
```

The job image compiles the playbook registry in. After adding or changing a playbook, rebuild it with `docker build -f docker/backtest/Dockerfile -t dagobert-backtest .`.

Target: about one minute for one year of 3-hour bars when the Parquet data is warm.

`task backtest-run BACKTEST_ID=<uuid>` still runs a single legacy backtest through the same job image. It has no UI page.

## Limits

- Results are simulated. They are not a claim of profitable trading.
- The simulation models spread, slippage and fees with fixed basis-point values, not the order book.
- A run needs Parquet coverage for the chosen range unless re-download is enabled.
