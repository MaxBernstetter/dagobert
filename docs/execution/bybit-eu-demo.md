# Execution: Bybit EU spot demo

Paper orders only. No deposit. The key is a demo key for a Bybit EU account.

Checked on 2026-10-20. A market buy of 5 USDC of `BTCUSDC` filled.

## Host

`https://api-demo.bybit.eu`

The key in `.secrets` is accepted only on this host. `https://api.bybit.eu` and `https://api-demo.bybit.com` both answer `retCode` `10003`, `API key is invalid`.

Credentials, from the environment, never from the repo:

| Variable | Use |
|---|---|
| `BYBIT_API_KEY` | Header `X-BAPI-API-KEY` |
| `BYBIT_API_SECRET` | HMAC key. Not sent. |

`.secrets` in the repo root exports both and is gitignored.

## Signature

Every private call:

```text
payload   = timestamp + api_key + recv_window + body
signature = hex(hmac_sha256(api_secret, payload))
```

`timestamp` is milliseconds as a decimal string. `recv_window` used here is `5000`. For `GET`, `body` is the query string without `?`. For `POST`, `body` is the compact JSON object, with no spaces.

Headers:

| Header | Value |
|---|---|
| `X-BAPI-API-KEY` | `BYBIT_API_KEY` |
| `X-BAPI-SIGN` | signature |
| `X-BAPI-TIMESTAMP` | timestamp |
| `X-BAPI-RECV-WINDOW` | `5000` |
| `Content-Type` | `application/json` on `POST` |

A response with `retCode` `0` and `retMsg` `OK` is success. Any other `retCode` is a failure. Do not retry a failure whose message says the request itself is invalid.

## Calls this slice uses

| Purpose | Request | On this host |
|---|---|---|
| Confirm the key | `GET /v5/account/info` | HTTP 200, `retCode` 0 |
| Place a spot order | `POST /v5/order/create` | HTTP 200, `retCode` 0 |
| Read the fill | `GET /v5/order/history?category=spot&orderId=...` | HTTP 200, `retCode` 0 |
| Current balance | `GET /v5/account/transaction-log?accountType=UNIFIED` | HTTP 200, `retCode` 0 |
| Wallet snapshot | `GET /v5/account/wallet-balance` | HTTP 404, empty body |

`GET /v5/account/wallet-balance` is not routed on `api-demo.bybit.eu`. It 404s for `UNIFIED`, `SPOT`, and `CONTRACT`. The balance is the `cashBalance` field on `GET /v5/account/transaction-log`.

```http
GET /v5/account/transaction-log?accountType=UNIFIED&limit=50
```

Do not pass `category=spot`. That filter hides the opening balance rows. Each row is one coin movement. `cashBalance` is the balance of `currency` after that movement. The current balance of a coin is the `cashBalance` on its newest row, the greatest `transactionTime`. Coins with no row are absent.

Checked after the 5 USDC buy:

| Currency | Latest `cashBalance` |
|---|---|
| USDC | `49995` |
| USDT | `50000` |
| BTC | `1.00006061` |
| ETH | `1` |

`GET /v5/account/info` returns the account mode. The checked account was `unifiedMarginStatus` `3`, `marginMode` `REGULAR_MARGIN`.

## Place a spot order

```http
POST /v5/order/create
```

```json
{
  "category": "spot",
  "symbol": "BTCUSDC",
  "side": "Buy",
  "orderType": "Market",
  "qty": "5",
  "marketUnit": "quoteCoin",
  "orderLinkId": "poc-1792511249"
}
```

| Field | Notes |
|---|---|
| `category` | `spot` |
| `symbol` | `BTCUSDC` or `ETHUSDC` |
| `side` | `Buy` or `Sell` |
| `orderType` | `Market` for this slice |
| `qty` | With `marketUnit` `quoteCoin`, this is the amount of USDC to spend. |
| `marketUnit` | `quoteCoin` so `qty` is quote, not BTC. |
| `orderLinkId` | Caller-chosen id. Use the candle-close key so a repeat does not create a second order. |

`BTCUSDC` on the demo host, checked the same day: status `Trading`, `minOrderQty` `0.000001`, `minOrderAmt` `1`, `basePrecision` `0.000001`. Five USDC is above the minimum.

Success body:

```json
{
  "retCode": 0,
  "retMsg": "OK",
  "result": {
    "orderId": "2330147682272051456",
    "orderLinkId": "poc-1792511249"
  }
}
```

## Read the fill

```http
GET /v5/order/history?category=spot&orderId=2330147682272051456
```

The fill is `result.list[0]`. Fields the execution service stores:

| Field | Example |
|---|---|
| `orderId` | `2330147682272051456` |
| `orderLinkId` | `poc-1792511249` |
| `symbol` | `BTCUSDC` |
| `side` | `Buy` |
| `orderType` | `Market` |
| `orderStatus` | `Filled` |
| `qty` | `5.0000000` |
| `cumExecQty` | `0.000060769216745565` |
| `cumExecValue` | `5.0000000` |
| `avgPrice` | `82278.5` |
| `marketUnit` | `quoteCoin` |

`cumExecQty` is the base asset received. `cumExecValue` is the quote asset spent. `avgPrice` is in USDC.

## Process

1. Read `BYBIT_API_KEY` and `BYBIT_API_SECRET` from the environment.
2. Take one pending action. Convert the USDT decision price with the stored `USDCUSDT` close: `usdc_price = usdt_close / usdcusdt_close`.
3. `POST /v5/order/create` with `orderLinkId` set to the action's candle-close key.
4. `GET /v5/order/history` with the returned `orderId`.
5. Store the fill fields above, or the `retCode` and `retMsg` when the order is rejected.
6. If `order/create` is called again with the same `orderLinkId`, do not treat a duplicate rejection as a second fill.

The ingest service does not call this host. This service does not read Binance klines.
