# Ingest: Binance public klines

No API key. No Binance account. Market data is public.

Checked on 2026-10-20 from this network. `GET /api/v3/klines` returned HTTP 200.

## Host

`https://data-api.binance.vision`

This host serves only public market data. The same paths exist on `https://api.binance.com`. Use the data host for ingest.

## Calls

| Purpose | Request |
|---|---|
| Closed candles | `GET /api/v3/klines` |
| Symbol rules | `GET /api/v3/exchangeInfo?symbol=BTCUSDT` |

Symbols for this slice: `BTCUSDT`, `ETHUSDT`, `USDCUSDT`.

### Klines

```http
GET /api/v3/klines?symbol=BTCUSDT&interval=1h&limit=2
```

| Parameter | Required | Notes |
|---|---|---|
| `symbol` | yes | |
| `interval` | yes | `1h` is the base interval. Build the 3-hour bar later, in the process service. |
| `startTime` | no | Open time in milliseconds, UTC. |
| `endTime` | no | Open time in milliseconds, UTC. |
| `limit` | no | Default 500, maximum 1000. |

Weight is 2 per request. Pace calls. On HTTP 429, back off. Repeated 429s can end in HTTP 418.

A request without `startTime` and `endTime` returns the most recent candles, including the candle that is still open. Keep a row only when its close time is already in the past.

Pagination: pass `startTime`, read up to 1000 rows, then set the next `startTime` to the last row's open time plus 1 millisecond. Stop when a page is short or passes `endTime`.

### One kline row

The body is a JSON array of arrays. Twelve fields, fixed order:

| Index | Name | Type | Store |
|---|---|---|---|
| 0 | open time | int, ms UTC | yes |
| 1 | open | decimal string | yes |
| 2 | high | decimal string | yes |
| 3 | low | decimal string | yes |
| 4 | close | decimal string | yes |
| 5 | volume | decimal string, base asset | yes |
| 6 | close time | int, ms UTC | yes |
| 7 | quote asset volume | decimal string | no |
| 8 | number of trades | int | no |
| 9 | taker buy base volume | decimal string | no |
| 10 | taker buy quote volume | decimal string | no |
| 11 | ignore | string | no |

A row checked live: `BTCUSDT` `1h`, open time `1792508400000`, close `82409.43000000`, volume `1069.70305000`, close time `1792511999999`.

### Symbol rules

`GET /api/v3/exchangeInfo?symbol=BTCUSDT` returns `symbols[0]`. Read `status` and `filters`.

| Filter | Field | `BTCUSDT` on 2026-10-20 |
|---|---|---|
| `PRICE_FILTER` | `tickSize` | `0.01000000` |
| `LOT_SIZE` | `stepSize` | `0.00001000` |
| `NOTIONAL` | `minNotional` | `5.00000000` |

`status` was `TRADING`. Read the rules when the ingest process starts. Do not store them as a dataset.

## Job

One run takes `From`, `To`, `Symbol`, and optional `-provider` (default `binance`, or `INGEST_PROVIDER`). `From` is inclusive and `To` is exclusive, both open times in milliseconds UTC. `Symbol` is one of `BTCUSDT`, `ETHUSDT`, or `USDCUSDT`.

The ingest **facade** owns the hour grid, ingest ledger (Postgres), lake writes, and HTTP fetch mechanics (`src/ingest/internal/fetchclient`). The **Binance adapter** (`src/ingest/provider/binance`) only builds one kline page request and maps the response. See [ingest-provider-facade](../grooming/ingest-provider-facade.md) and [ingest-fetch-facade-refactor](../grooming/ingest-fetch-facade-refactor.md).

The run lists open times already stored for that provider+symbol inside the range and fetches only the gaps. A second run with the same parameters makes no Binance call and writes no rows.

### Facade transport (env)

Shared across providers: `max_retries`, `backoff_factor`, `retry_wait`, `max_retry_wait`, `rate_limit` (request weight per minute), `concurrency` (semaphore). The facade retries when the adapter reports `Retryable`; it honors `Retry-After` from the page result when present.

### Binance adapter (provider-only)

| Setting | Value |
|---|---|
| Connection | `BINANCE_BASE_URL` only |
| Route | `GET /api/v3/klines` (`interval=1h`, `limit` ≤ 1000) |
| Request weight | 2 (facade acquires this per page against `rate_limit`) |
| Max page | 1000 hours (API candle limit) |
| Retryable | 429, 418, 5xx; transport/parse errors on status 0 or 200. Not 400. |

## Process

1. Start with `From`, `To`, `Symbol`, and provider (default `binance`).
2. Skip every open time in that range that is already stored or marked unavailable for that provider.
3. Fetch the missing closed `1h` klines. Drop a row whose close time is still in the future.
4. Write the seven stored fields to Hive-style Parquet (columns as in [Candle file format](#candle-file-format)): `parquet/provider={id}/base={BASE}/quote={QUOTE}/year={YYYY}/candles.parquet`, with footer key–value metadata `trading_copilot.schema_version` (`2`), `trading_copilot.interval_ms` (here `3600000` for `1h`) and `trading_copilot.data_source` (`binance`, see `src/internal/parquetmeta` and `src/shared/datasource.go`). Index rows in the Postgres table `ingest_stored` with PK `(provider, base, quote, open_time)`.
5. Do not compute indicators. Do not call any other exchange.

`USDCUSDT` is ingested because the execution service converts a USDT close into USDC with `usdc_price = usdt_close / usdcusdt_close`.

## Candle file format

Schema version 2 (DATA.01). The Binance decimal strings are stored as exact fixed-scale decimals, never as floats.

| Column | Parquet type | Encoding |
|---|---|---|
| `open_time`, `close_time` | INT64 | DELTA_BINARY_PACKED, Snappy |
| `open`, `high`, `low`, `close` | DECIMAL(18,8) on INT64 | DELTA_BINARY_PACKED, Snappy, no dictionary |
| `volume` | DECIMAL(28,8) on FIXED_LEN_BYTE_ARRAY(12) | PLAIN, Snappy, no dictionary |

Scale 8 is Binance's maximum precision. Prices fit DECIMAL(18,8) (below 1e10). Volume is in base-asset units: a high-supply token such as SHIB or PEPE trades 1e11 to 1e12 units per hour, which overflows DECIMAL(18,8), so volume gets DECIMAL(28,8) (up to about 1e20). Snappy is used because the C++ reader (Arrow) is built with Snappy and zlib only.

Ingest never truncates. A value with more than 8 significant decimals (trailing zeros are fine), with an exponent, or that does not fit its precision makes the run fail with exit 1 and `DETAIL=50`; the log names the open time and the value, and nothing is stored for that run.

Schema version 1 (decimal strings in `open`..`volume`) is still readable: every reader detects the column type. Writers only write version 2. Convert an existing lake with `task data:migrate-candles` (`DATA_DIR`, default `./data`): it rewrites each version 1 file through a temp file and rename after a decimal round-trip check, skips files already converted, and prints `migrated=… skipped=… failed=… rows=…`.
