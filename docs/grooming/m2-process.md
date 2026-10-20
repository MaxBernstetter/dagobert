# M2 — Process

First technical overview. Not a design. Parameters, schemas, and warm-up rules come in a later grooming pass.

## What it is

Process is its own Go service and its own container. It is not the ingest service.

For one closed 3-hour UTC candle it reads the Parquet lake from M1, builds that bar from the stored 1-hour klines, and writes one KPI row per symbol.

It does not call Binance. It does not call Bybit. It does not place orders, run a playbook, or decide a trade.

## Where it runs

The Go binary lives in a Docker image and is only the runner. It has no server and no state of its own. A Lambda invocation starts one container, the process runs, and the container exits.

Locally, Floci stands in for that Lambda and for DynamoDB. On AWS the Lambda starts this container when the process step is due. The Lambda does not read Parquet and does not compute the row. Process does not run inside the ingest container.

The invocation is stateless. Everything it needs is in the parameters and in Parquet. Nothing is kept in memory or on the container disk for a later run.

## Unit of work

One invocation is one symbol and one closed 3-hour bar on a fixed UTC boundary (00:00, 03:00, 06:00, …).

| Parameter | Meaning |
|---|---|
| `Symbol` | `BTCUSDT`, `ETHUSDT`, or `USDCUSDT` |
| `BarOpen` | Open time of the 3-hour bar, ms UTC |

Ingest stores closed Binance `1h` klines only. The 3-hour bar is built here, from those rows, and only after the bar’s close time is in the past.

The invocation is idempotent. If the KPI row for that symbol and `BarOpen` is already stored, the runner exits. It does not read the extra history and it does not write again. A repeated invocation does not insert a second row.

`USDCUSDT` is a price input for a later USDT-to-USDC conversion. It still gets a KPI row.

## Input

Parquet from M1, partitioned by symbol. Seven stored fields per closed 1-hour kline:

| Field | Meaning |
|---|---|
| open time | ms UTC |
| open | decimal |
| high | decimal |
| low | decimal |
| close | decimal |
| volume | base asset |
| close time | ms UTC |

Quote volume, trade count, and taker-buy fields are not in the lake. Process does not fetch them.

## Output

M3 defines and owns one general database schema. M2 writes one row that matches it and does not change it. There is not a table, item type, or file per indicator.

One KPI row per symbol per closed 3-hour candle. The row is the only input the later playbook reads. Identity is symbol plus the 3-hour bar open time (UTC).

The row carries the constructed bar and a list of indicator results. It does not have a column per nickname (`sma_fast`, `sma_slow`). Two moving averages are one definition with different parameters. The definition is in `docs/indicators/schema.md`. Each type has its own note under `docs/indicators/`.

### Bar

`open`, `high`, `low`, `close`, `volume`, `open_time`, `close_time`

### Indicator result

```json
{"type": "MovingAverage", "method": "sma", "time_frame_hr": 3}
```

The result is stored next to that object, on the same general row. The same object with `time_frame_hr` 6 is a second result, not a second definition and not a second table.

## Calculation graph

The process builds a graph of these parameter objects for one symbol and one bar. Two nodes with the same normalized object are one node. A node runs only after the nodes it depends on, and it passes that result on.

An SMA of 6 hours and an SMA of 12 hours share the moving-average definition. The longer one reuses the shorter sum where the windows overlap. It does not walk the same bars twice. EMA is the same definition with `method` `ema`. Close-distance and Bollinger mid take the SMA node. MACD takes the EMA nodes. Volume comparison takes the volume-mean node.

Normalization is part of the schema, not a cleanup step afterwards. `time_frame_hr` is a positive multiple of 3, because the bar is 3 hours. The type declares the unit of its value (`price`, `amount`, `ratio`, or `index`). A playbook compares values only when the unit matches. A price is not compared with an RSI, and a volume is not compared with a price.

## What a later playbook uses them for

Trend rules read the moving averages, their distances, and MACD. Breakout rules read the Donchian columns. Momentum rules read the return columns. Mean-reversion rules read RSI and the Bollinger columns. Exit and size overlays read ATR and realized volatility. Volume columns are context, not a separate strategy.

Process does not apply those rules.

## Exit codes

The container exits with one general code. That is the only process status. The job also reports a detail code that M2 owns. The Lambda records both. Logs are not the outcome. Any other process status is a defect in the runner.

| Code | Outcome |
|---|---|
| 0 | Succeeded. |
| 1 | Failed. |
| 2 | Aborted. |

| Detail | General | Outcome |
|---|---|---|
| 0 | 0 | The KPI row was written on this run. |
| 1 | 2 | Aborted. The KPI row for `Symbol` and `BarOpen` is already stored. No recompute and no write. |
| 10 | 1 | `Symbol` or `BarOpen` is missing or invalid. |
| 20 | 2 | The bar is still open. |
| 21 | 2 | One or more 1-hour candles for that bar are not stored yet. |
| 30 | 1 | Reading Parquet or writing the row failed after the configured retries. |
| 40 | 1 | The Parquet for that bar does not match the schema M2 owns. |

## Persistence

M2 owns its own store and does not open another job's. Locally that is `process.sqlite`. On AWS it is one DynamoDB on-demand table. The runner only reads by key and writes if that key is absent. The key is `Symbol` plus `BarOpen`. A hit is exit `2` with detail `1`: no recompute and no second row. The item is one general record for that symbol and bar. It holds the bar and every indicator value. M3 does not define a store per indicator.

## Acceptance

Proved only by system tests on the finished process container. The tests do not import the Go packages. The fixture is a Parquet file. The outcome is a fixed KPI row.

- A fixture of three closed 1-hour candles at a 3-hour boundary writes one general KPI row and exits `0` / detail `0`.
- The same bar again exits `2` / detail `1` and does not recompute.
- A bar that is still open exits `2` / detail `20`.
- A bar whose 1-hour candles are missing exits `2` / detail `21`.
- A missing `BarOpen` exits `1` / detail `10`.
- Two `MovingAverage` nodes that share a window do not walk those bars twice. The test asserts the stored results, not a call count inside the process.

## Open questions

- Key and item shape in DynamoDB. The repeated run must be able to see that the row exists without recomputing it.
- Numeric type and rounding for indicator columns.
- Lookback lengths, and how a bar is skipped when warm-up history is missing.
- Whether `USDCUSDT` keeps the full indicator set or only the bar close.
