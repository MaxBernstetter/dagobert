# Ingestion: Binance integration

Status: implemented. Binance is the only market data provider.

## What it does

The ingest job fetches closed 1-hour spot candles from Binance and writes them to the Parquet lake. It takes no API key and uses only public market data. It computes no indicators and does not talk to the trading venue.

The 3-hour strategy bars are built later, by the processing stage, from the validated 1-hour base.

## Endpoints

Host: `https://data-api.binance.vision`. It serves public market data only. The same paths exist on `https://api.binance.com`. Ingest uses the data host.

| Purpose | Request |
|---|---|
| Closed candles | `GET /api/v3/klines?symbol=…&interval=1h&startTime=…&limit=1000` |
| Symbol rules | `GET /api/v3/exchangeInfo?symbol=…` |
| Pair list (orchestrator) | `GET /api/v3/exchangeInfo` for all spot markets with status `TRADING` |

Request weight is 2 per kline call. The facade paces calls against `rate_limit`. A response of 429 or 418 is retried with backoff, and `Retry-After` is honoured when present. Repeated 429s can end in 418, so the job does not retry faster than the limit.

### Pagination

Pass `startTime`, read up to 1000 rows, then set the next `startTime` to the last row's open time plus 1 ms. Stop when a page is short or passes `endTime`.

### Closed candles only

A request without a time range returns the most recent candles, including the one still open. A row is kept only when its close time is in the past.

## Kline row

Each row is a fixed-order array of 12 fields. The job stores the first seven:

| Index | Field | Stored |
|---|---|---|
| 0 | open time (ms, UTC) | yes |
| 1 | open | yes |
| 2 | high | yes |
| 3 | low | yes |
| 4 | close | yes |
| 5 | volume (base asset) | yes |
| 6 | close time (ms, UTC) | yes |
| 7–11 | quote volume, trade count, taker buy volumes, ignore | no |

Prices and volumes arrive as decimal strings. They are validated and stored as fixed-scale decimals (scale 8), see [Candle file format](binance-public-klines.md#candle-file-format).

## Symbol rules

The adapter reads `status` and `filters` from `exchangeInfo` when the job starts: `PRICE_FILTER.tickSize`, `LOT_SIZE.stepSize` and `NOTIONAL.minNotional`. These are read, not stored as a dataset.

## Scope

The job takes `-base` and `-quote` (for example `BTC` and `USDT`) with `-from` and `-to` open times in milliseconds UTC. `-from` is inclusive and `-to` is exclusive. `-provider` defaults to `binance`. `-force` re-fetches every closed hour in the range and replaces the stored rows.

The legacy `-symbol BTCUSDT` flag is still accepted and split into base and quote.

The pipeline runs on `BTC/USDT`, `ETH/USDT` and `USDC/USDT`. `USDC/USDT` is needed because execution converts a USDT price into USDC with `usdc_price = usdt_close / usdcusdt_close`. The orchestrator's pair list shows every TRADING spot market, but the pipeline only processes the pairs configured for it.

## Process

1. List the open times already stored or marked unavailable for this provider, base and quote in the range.
2. Fetch only the missing closed hours, page by page.
3. Drop any row whose close time is still in the future.
4. Write the seven stored fields to Parquet at `parquet/provider={id}/base={BASE}/quote={QUOTE}/year={YYYY}/candles.parquet`. The footer carries `trading_copilot.interval_ms` (3600000) and `trading_copilot.data_source` (`binance`).
5. Index each stored hour in the Postgres table `ingest_stored` with primary key `(provider, base, quote, open_time)`.

A second run with the same parameters makes no Binance call and writes no rows.

## Exchange gaps

Ingest never synthesises a candle. When Binance returns no closed candle for an hour, that open time is recorded in the `unavailable` table with a reason. These hours are not retried by a normal run. `-force` clears them so they are fetched again.

Only retryable failures block a run. A transport failure after the retry budget is a retryable gap; an unavailable hour is not.

## Exit codes

| Exit | Meaning |
|---|---|
| `0` | At least one hour was written, possibly with unavailable holes recorded. |
| `1` | Failed. The detail code says why. |
| `2` | Nothing to do: every closed hour is already stored or unavailable, or an hour in the range is still open. |

Detail codes: `10` invalid input or provider, `20` an hour in the range is still open, `30` Binance transport failed after retries, `32` no new candles and only holes were recorded, `40` data layout or SQLite inconsistency.

## Retry and transport

The shared facade owns the hour grid, the SQLite index, the lake writes and the HTTP mechanics. The Binance adapter only builds one kline request and maps the response. Facade settings are `max_retries`, `backoff_factor`, `retry_wait`, `max_retry_wait`, `rate_limit` and `concurrency`.

| Response | Retried |
|---|---|
| 429, 418, 5xx | yes |
| transport error, or a parse error on status 200 | yes |
| 400 and other 4xx | no |

Redirects are followed only within the same host, up to two hops.

## Configuration

| Variable | Use |
|---|---|
| `BINANCE_BASE_URL` | Base URL. Defaults to `https://data-api.binance.vision` in backtest runs. |
| `INGEST_PROVIDER` | Provider id when `-provider` is not given. |
| `DATA_DIR` | Root of `parquet/` when `LAKE_BACKEND=fs`. |
| `DATABASE_URL` | Postgres holding the ingest ledger (`ingest_stored`, `ingest_unavailable`). Required; the run fails (exit 1) without it. |
| `LAKE_BACKEND` | `fs` (default for the bare job) or `s3`. With `s3` the job uploads each year file to `LAKE_BUCKET` (at `AWS_ENDPOINT_URL` on Floci), keeps no local copy, and fails the run if the upload fails. Compose sets `s3`. |
| `LAKE_BUCKET` | Bucket for `s3`. |

Related: [backtesting](../architecture/backtesting.md)
