# Seed one year of demo history (BTC)

Wipes `./data`, ingests **one year** of Binance public klines for `BTCUSDT`, backfills KPIs, then runs **ma-cross** for every closed 3-hour bar: signal → market-action → execution.

Prefer `task up` / `task seed-demo-year` (they create `data/parquet` on the host and pass your UID/GID to the orchestrator). If older runs left root-owned files under `data/`, the seed script clears them with a one-off `docker run … alpine find /data -mindepth 1 -delete`, or run `sudo chown -R "$USER:$USER" data`.

Venue calls are **not** made when `VENUE_SIMULATE=1` (set by the seed script). Balances and fills use `data/paper_venue.json` and bar **close** prices from `technical_indicator.sqlite`.

```bash
task seed-demo-year
```

For **non-destructive** experiments over historical data, use the **Experiments** page instead (see `docs/development/run-backtest.md`).

Requires network for Binance ingest. Ingest and KPI backfill dominate runtime; the bar replay uses **`FastSimulatedPipeline`**: KPI series for the window are loaded once, signal/market-action/execution run in memory (paper wallet), then signal/action rows and execution tables are flushed in batched transactions (not one DB open per bar). Restart the frontend if it is already running so it picks up the new `data/` volume.

Live trading still uses Bybit when `VENUE_SIMULATE` is unset.
