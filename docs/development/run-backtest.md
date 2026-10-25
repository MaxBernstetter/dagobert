# Run a backtest

After pulling backtest-related changes, rebuild the orchestrator so `POST /backtest/run` is available:

```bash
docker compose build orchestrator && docker compose up -d orchestrator
task build:jobs   # includes dagobert-backtest
```

After adding or changing a **playbook**, rebuild the backtest job (the playbook registry is compiled into the image):

```bash
docker build -f docker/backtest/Dockerfile -t dagobert-backtest .
```

Re-run **orchestrator** after backtest-runner changes so job containers are started with the right Docker flags (the backtest job must be able to write `data/backtests.sqlite`).

**Market data:** every run **fills gaps** in `data/parquet` for the chosen range (missing hours only). Turn on **Force re-download** to replace existing candles in that range. With warm parquet, stage 2 is usually quick; a forced full-year refresh can sit on stage 2 for a long time while progress looks stuck near 25%.

**Lake layout cutover (provider / base / quote / year + footer meta):** after pulling Hive or metadata changes, **wipe** local candle state — remove `data/parquet/` and `data/ingest.sqlite` (and recreate the stack data dir as needed). Old `symbol=` trees and `.meta.json` sidecars are not migrated.


1. Start the stack (`task up`) and build job images once (`task build:jobs`).
2. Open **Experiments** in the UI (home redirects there), create an experiment, then **Run experiment**.

Or from the CLI (copy the experiment id from the list):

```bash
task experiment-run EXPERIMENT_ID=<uuid>
```

The experiment job pre-checks parquet coverage, builds KPI **once in memory**, simulates each instance, then writes `experiment_kpi` and workspace results at the end. Orchestrator exposes `POST /experiment/run` (single-flight shared with backtest jobs).

Requires `data/parquet` to cover the chosen range unless **Re-download missing candles** is enabled. Target runtime is under one minute for ~1y of 3h bars when Parquet is warm.

### Simulation settings (snapshotted per run)

| Setting | Env (during job) | Default |
|---------|------------------|---------|
| Initial USDC / USDT | `paper_venue.json` | 100k / 0 (quote is USDC for BTC/ETH) |
| Target weight % | `ALLOC_TARGET_WEIGHT_PCT` | 95 (long sleeve when in market) |
| Cash reserve | `ALLOC_CASH_RESERVE_QUOTE` | 100 |
| Min order (quote) | `ALLOC_MIN_ORDER_QUOTE` | 1 |
| Spread (bps, full width) | `SIM_SPREAD_BPS` | 4 |
| Slippage (bps) | `SIM_SLIPPAGE_BPS` | 2 |
| Fee (bps) | `SIM_FEE_BPS` | 6 |

**Scenario:** prefer `task seed-demo-year` (or existing `data/parquet` + KPI lake). You can set `from` earlier than the seeded window; the job will ingest missing hours unless force re-download is on. **Performance** in the UI reports total return vs **buy-and-hold** (100% BTC from the first bar, same starting USDC).

Reference price is the **3h bar close** from backtest KPI marks. **Buy** fills at `close × (1 + spread/2 + slippage)` and **sell** at `close × (1 − spread/2 − slippage)` (all in bps). Fees apply to quote notional after that price. Live simulated execution (`VENUE_SIMULATE=1`) uses the same `SIM_*` variables when set.

For a full demo reset of live data, use `task seed-demo-year`; for non-destructive runs use **Experiments**.
