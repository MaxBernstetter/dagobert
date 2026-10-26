# M4 — Paper fill

Execution service. Venue details live in [bybit-eu-demo.md](../../execution/bybit-eu-demo.md). What the trader sees is [m4-ux.md](m4-ux.md).

## Service boundary

Own Go binary, own image, started the same way as ingest, process, and strategy: a Lambda starts the container. The container is the runner.

Invocation: `Symbol` and `CandleClose` (UTC milliseconds). `PLAYBOOK` is the playbook file name, the same identity M3 stored (`ma-cross.yaml`). `DATA_DIR` holds `strategy.sqlite` (read) and `execution.sqlite` (write).

It reads one action row. It does not read Parquet, does not read KPI rows, and does not evaluate rules. It talks only to `https://api-demo.bybit.eu`. Credentials are `BYBIT_API_KEY` and `BYBIT_API_SECRET`. The secret is the HMAC key and is never sent. `recv_window` is `5000`. `retCode` `0` is success.

## The action it reads

M3 already wrote the row. Key: playbook basename, symbol (`BTCUSDT` or `ETHUSDT`), candle close. Body: `decision` (`buy`, `sell`, `no-trade`), `size`, `exit`, `rule`.

`orderLinkId` is `{playbook}:{symbol}:{candleClose}`. That string is the execution key and the Bybit client id.

Venue symbol is `BTCUSDC` or `ETHUSDC`. M4 does not convert a USDT price. `size` on a buy is USDC to spend. `size` on a sell is base quantity. The playbook declares those units.

## One run

1. Reject a missing or invalid symbol or candle close (`1` / `10`).
2. Read the action. No row means the strategy has not written it (`2` / `20`). No Bybit call.
3. Look up `orderLinkId`.
   - A stored fill, rejection, or no-trade is already settled (`2` / `1`). No order, no history poll.
   - A stored order that is not filled yet is polled again. No second `order/create`.
4. `decision` `no-trade`: refresh the balance snapshot, write the settled no-trade, exit `0` / `2`. No `order/create`.
5. `buy`: `POST /v5/order/create` with `category` `spot`, venue symbol, `side` `Buy`, `orderType` `Market`, `marketUnit` `quoteCoin`, `qty` the USDC `size`, `orderLinkId` as above.
6. `sell`: the same call with `side` `Sell`, `marketUnit` `baseCoin`, `qty` the base `size`.
7. `retCode` other than `0`: store `retCode` and `retMsg`, exit `1` / `40`. A retry of a stored rejection is `2` / `1`.
8. `retCode` `0`: `GET /v5/order/history?category=spot&orderId=...`. `Filled` stores the fill and exits `0` / `0`. Any other status stores the order and exits `2` / `31`. The next run polls that order.
9. Refresh the balance snapshot after a fill or a rejection that reached Bybit.

A duplicate `order/create` for the same `orderLinkId` is not a second fill. The local key is checked before the call so the happy path never sends it twice.

## What is stored

One item per `orderLinkId` in `execution.sqlite`. On AWS, one DynamoDB on-demand table. Write if the key is absent. An unfilled order may be updated in place when history later says `Filled`. A settled item is never replaced.

| Kind | Fields |
|---|---|
| Fill | `orderId`, `orderLinkId`, playbook, symbol, venue symbol, candle close, `side`, `orderStatus`, `qty`, `cumExecQty`, `cumExecValue`, `avgPrice`, `marketUnit` |
| Rejection | `retCode`, `retMsg`, plus the key fields. No fill. |
| No-trade | The key fields and `decision` `no-trade`. No order id. |
| Not filled | `orderId`, `orderStatus`, plus the key fields. |

The rule text stays on the strategy action. This item links to it by playbook, symbol, and candle close.

Balance is a separate record in the same store: one current `cashBalance` and `transactionTime` per currency, replaced on each run that reads the log. Coins with no log row stay absent. Source is `GET /v5/account/transaction-log?accountType=UNIFIED` with no `category` filter. `GET /v5/account/wallet-balance` is HTTP 404 on this host and is not called.

## Fetch client

Same shape as ingest, for Bybit only: `max_retries`, `backoff_factor`, `retry_wait`, `max_retry_wait`. Retry HTTP 429, 5xx, and a `retCode` that is a rate limit or a transient server error. Do not retry a rejection of the order itself (`1` / `40`). Exhausted retries are `1` / `30`. Settings come from the environment.

## Exit codes

The container exits with one general code. The job prints `DETAIL=<n>`. The Lambda records both.

| Code | Outcome |
|---|---|
| 0 | Succeeded. |
| 1 | Failed. |
| 2 | Aborted. |

| Detail | General | Outcome |
|---|---|---|
| 0 | 0 | A fill was stored on this run. |
| 1 | 2 | This `orderLinkId` is already settled. No second order. |
| 2 | 0 | Explicit no-trade. Nothing was sent. The snapshot was refreshed. |
| 10 | 1 | Symbol or candle close is missing or invalid. |
| 20 | 2 | The action for that candle is not stored yet. |
| 30 | 1 | Bybit failed after the configured retries. |
| 31 | 2 | The order was accepted and is not filled yet. |
| 40 | 1 | Bybit rejected the order. The rejection is stored. |

## Acceptance

System test against the finished image. The test does not import the service packages. Fixture is a seeded action row in `strategy.sqlite` and a faked Bybit HTTP server. Outcome is the exit codes, the execution row, and the balance rows.

| Case | Result |
|---|---|
| Buy action, create and history both `Filled` | `0` / `0`. One fill. One `order/create`. |
| Same args again | `2` / `1`. No `order/create`. |
| No-trade action | `0` / `2`. No `order/create`. Snapshot written. |
| Sell action | `0` / `0`. `marketUnit` `baseCoin`, `qty` is the base size. |
| No action row | `2` / `20`. No Bybit call. |
| Missing symbol | `1` / `10`. |
| Create returns a permanent `retCode` | `1` / `40`. Rejection stored. Repeat is `2` / `1`. |
| Create succeeds, history is not `Filled` | `2` / `31`. Repeat polls history and does not create. |
| HTTP 500 until retries run out | `1` / `30`. |

Audit after the implementation PR: unfilled-then-filled, a sell below `minOrderQty`, snapshot replace (not append), and a crash between create and the local write.
