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

One run takes `From`, `To`, and `Symbol`. `From` is inclusive and `To` is exclusive, both open times in milliseconds UTC. `Symbol` is one of `BTCUSDT`, `ETHUSDT`, or `USDCUSDT`.

The run lists open times already stored for that symbol inside the range and fetches only the gaps. A second run with the same parameters makes no Binance call and writes no rows.

Fetching is one client. These settings come from the environment: `max_retries`, `backoff_factor`, `retry_wait`, `max_retry_wait`, `rate_limit` (request weight per minute; a kline call weighs 2), and `concurrency` (a semaphore). Retry 429, 418, and 5xx. Honor `Retry-After`. Do not retry a 400.

## Process

1. Start with `From`, `To`, and `Symbol`.
2. Skip every open time in that range that is already stored.
3. Fetch the missing closed `1h` klines. Drop a row whose close time is still in the future.
4. Write the seven stored fields to Parquet, partitioned by symbol. Beside each `symbol.parquet` file, write `symbol.parquet.meta.json` with `interval_ms` (here `3600000` for `1h`) and `data_source` (`binance`, see `src/shared/datasource.go`).
5. Do not compute indicators. Do not write the database. Do not call Bybit.

`USDCUSDT` is ingested because the execution service converts a USDT close into USDC with `usdc_price = usdt_close / usdcusdt_close`.
