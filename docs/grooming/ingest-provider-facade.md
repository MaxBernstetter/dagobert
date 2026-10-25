# Ingest provider facade

Separate market-data **providers** from the ingest **facade** so multiple exchanges can coexist in one lake without mixing rows.

## Decisions

| Decision | Choice |
|----------|--------|
| First adapter | Binance public 1h klines (existing behaviour) |
| Selection | One provider per ingest run (`-provider` / `INGEST_PROVIDER`, default `binance`) |
| Lake layout | Hive-style under `DATA_DIR/parquet/` |
| Partition keys | `provider`, `base`, `quote`, `year` (UTC; no month/timeframe yet) |
| Object name | `candles.parquet` per year partition; metadata in Parquet footer KV |
| Mixing | Never mix providers in one parquet file |

## Layout

```text
DATA_DIR/
  parquet/
    provider=binance/
      base=BTC/
        quote=USDT/
          year=2024/
            candles.parquet   # footer: trading_copilot.* keys
  ingest.sqlite   # PK (provider, base, quote, open_time)
```

Footer metadata schema (`src/internal/parquetmeta`): `trading_copilot.schema_version`, `trading_copilot.interval_ms`, `trading_copilot.data_source`.

See [ingest-list-assets](ingest-list-assets.md) for pair discovery and cutover (wipe old layouts).

## Facade vs adapter

- **Facade:** hour grid, skip stored/unavailable, merge-write parquet, sqlite index, exit/detail codes, path helpers, and **fetch transport** (`internal/fetchclient`: window split, semaphore, weight limiter, retry/backoff from env).
- **Adapter:** `FetchPage` (one HTTP attempt) + policy (`MaxPageHours`, `RequestWeight`, `Retryable`), provider id / `DataSource`, empty-hole reason. No retry/semaphore/limiter in the adapter. See [ingest-fetch-facade-refactor](ingest-fetch-facade-refactor.md).

## Callers

Jobs that only know `dataDir` + `symbol` resolve provider via `instruments.BySeries(symbol).PriceSource` (today `"binance"`).

## Follow-ups (out of scope)

- Additional exchange adapters
- Extra Hive keys (`month`, `timeframe`) — month trades more files / list overhead for ~1h bars vs Athena prune wins
- Provider as experiment GRID axis (experiment-level provider is in the UI)

## Tracking

Epic/sub-issues were not opened from this environment (`gh` unavailable). Create them on `MaxBernstetter/dagobert` from this doc when the GitHub MCP / `gh` is available.
