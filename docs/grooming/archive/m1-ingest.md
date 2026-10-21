# M1 — Ingest

First technical overview. Binance public klines are the only market-data source. No API key and no Binance account.

## What it is

Ingest is its own Go service in its own container. One run fetches closed 1-hour klines and writes them to Parquet on S3.

Symbols for this slice: `BTCUSDT`, `ETHUSDT`, `USDCUSDT`. `USDCUSDT` is stored so a later service can convert a USDT close into USDC. Ingest does not do that conversion.

The base interval is `1h`. Building a 3-hour bar is not this service's job.

## What it does not do

- It does not compute KPIs, indicators, or features.
- It does not write a database.
- It does not call Bybit or place orders.
- It does not talk to any other market-data vendor.

## Inputs

Public REST on `https://data-api.binance.vision` (not `api.binance.com`):

| Call | Use |
|---|---|
| `GET /api/v3/klines` | Closed candles. `interval=1h`. |
| `GET /api/v3/exchangeInfo?symbol=…` | Read `status` and filters when the process starts. Do not store them as a dataset. |

A kline request without `startTime` and `endTime` includes the candle that is still open. Keep a row only when its close time is already in the past. Weight is 2 per kline request.

Each run receives three parameters:

| Parameter | Meaning |
|---|---|
| `From` | First open time to consider, inclusive, ms UTC. |
| `To` | Last open time to consider, exclusive, ms UTC. |
| `Symbol` | One symbol: `BTCUSDT`, `ETHUSDT`, or `USDCUSDT`. |

One run is one symbol. Three symbols are three runs. The S3 endpoint and bucket come from the environment. No Binance secret.

## Output and Parquet contract

M2 defines and owns the Parquet schema. M1 writes rows that match it and does not change it.

Object storage only. Another service can rely on this:

- One object family per symbol, for closed `1h` bars from Binance spot.
- A row is unique by symbol and open time. Writing the same open time again does not create a second logical candle.
- Seven fields, fixed meaning, UTC timestamps in milliseconds:

| Field | Meaning |
|---|---|
| `open_time` | Candle open, int, ms UTC |
| `open` | Decimal price |
| `high` | Decimal price |
| `low` | Decimal price |
| `close` | Decimal price |
| `volume` | Base-asset volume, decimal |
| `close_time` | Candle close, int, ms UTC |

Quote volume, trade count, and taker-buy fields are not part of the contract.

Partition at least by symbol so a reader can load one symbol without scanning the others. Exact key layout, Parquet physical types, and whether each hour is its own object are open (below).

## Runtime

- The service runs as its own Go container. It is not in the same process as process, strategy, or execution.
- Locally, Floci provides S3. The container talks to that endpoint.
- On AWS, a Lambda starts that container and passes `From`, `To`, and `Symbol`. The Lambda does not fetch klines or write Parquet. Scheduling across services is M5; M1 only needs this start path for ingest.

## One run

1. Start the container with `From`, `To`, and `Symbol`.
2. List the open times already stored for that symbol inside `[From, To)`. Those hours are done. Do not fetch them and do not write them again.
3. If nothing in the range is missing, exit successfully. No Binance call, no Parquet write.
4. Fetch only the missing open times. Drop a row whose close time is still in the future, and drop a row outside `[From, To)`.
5. Write the new rows. A second run with the same parameters writes nothing.
6. Exit. Success means every closed hour in the range is stored. An hour that is still open is not a success.

The same run can be repeated after a crash. Hours already stored are skipped. A partial write of an hour is finished by the next run, not duplicated.

## Fetching

The Binance call sits behind one client with a fixed set of limits. The job does not invent a second retry path.

| Setting | Role |
|---|---|
| `max_retries` | Attempts for one page after the first failure. |
| `backoff_factor` | Multiplier applied to the wait between attempts. |
| `retry_wait` | Wait before the first retry. |
| `max_retry_wait` | Cap on that wait. |
| `rate_limit` | Maximum request weight per minute. A kline call weighs 2. |
| `concurrency` | How many page fetches may be in flight. A semaphore holds the rest. |

Retry HTTP 429, 418, and 5xx. Honor `Retry-After` when Binance sends it. Do not retry a 400. The settings come from the environment so a run can be tightened without a code change.

Pages advance by open time: `startTime` moves to the last returned open time plus 1 millisecond, and `limit` stays at most 1000. Only pages that overlap a missing hour are requested.

## Exit codes

The container exits with one general code. That is the only process status. The job also reports a detail code that M1 owns. The Lambda records both. Logs are not the outcome. Any other process status is a defect in the runner.

| Code | Outcome |
|---|---|
| 0 | Succeeded. |
| 1 | Failed. |
| 2 | Aborted. |

| Detail | General | Outcome |
|---|---|---|
| 0 | 0 | The missing closed hours were written on this run. |
| 1 | 2 | Aborted. Every closed hour in `[From, To)` is already stored. No Binance call and no write. |
| 10 | 1 | `From`, `To`, or `Symbol` is missing or invalid. |
| 20 | 2 | A candle in the range is still open, so the range cannot be finished. |
| 30 | 1 | Binance failed after the configured retries. |
| 40 | 1 | A stored object for an hour in the range is partial and cannot be trusted. |

## Persistence

M1 owns its own store and does not open another job's. Locally that is `ingest.sqlite`. On AWS it is one DynamoDB on-demand table. The runner only reads by key and writes if that key is absent. The key is symbol plus open time. A hit is exit `2` with detail `1`: no Binance call and no second write. Parquet remains the candle data, in the schema M2 owns. The SQLite file and the DynamoDB table record which open times are already stored.

## Acceptance

Proved only by system tests on the finished ingest container. The tests do not import the Go packages. The fixture is a faked Binance HTTP response. The outcome is fixed.

- A range with two closed hours writes those two rows and exits `0` / detail `0`.
- The same range again exits `2` / detail `1` and does not call Binance.
- A range that includes the still-open hour exits `2` / detail `20`.
- A missing `Symbol` exits `1` / detail `10`.
- A fixture that returns HTTP 500, then a valid page, succeeds within the configured retries.
- A fixture that returns HTTP 500 until retries are exhausted exits `1` / detail `30`.

The Binance request shape is also recorded as a Bruno collection. That collection does not replace the container tests.

## Open questions

- Object key and partition layout (hour object vs larger files, symbol naming, year prefixes). The layout in `docs/IDEA.md` is illustrative.
- Parquet physical types for the two timestamps and five decimals, and the schema version a reader should expect.
- How a crashed write of one hour is detected so the next run fetches that hour again instead of trusting a partial object.
- Whether a non-`TRADING` symbol fails the run or is skipped.
- The numeric defaults for the retry and rate-limit settings.
