# Research: trading execution

Decision for the first place the copilot can send orders. Checked on 2026-10-20. The requirement is algorithmic orders against artificial money, with no deposit.

## Recommendation

Use **Binance Spot Demo Mode** (`https://demo-api.binance.com`).

It is the live spot API pointed at a paper account. Prices and the book follow the live exchange, filters and rate limits match production, and the balance is virtual. Resetting the balance is a UI action, not a monthly wipe. Authentication is an API key created from Demo Trading on a Binance account. Market-data calls can stay on the public production host; only signed order and account calls go to the demo host.

If a Binance account is not available in the operator's country, use **Bybit Demo Trading** (`https://api-demo.bybit.com`) the same way: a separate demo key, live public prices, virtual funds.

## What this does not cover

- A guarantee that a strategy which works in demo will work live. Binance says demo prices are realistic and are not the production matching engine. See [Demo Mode general info](https://developers.binance.com/docs/binance-spot-api-docs/demo-mode/general-info).
- Futures, margin, or withdrawals. This note is spot only. Demo balances cannot be moved out.
- Regions where Binance or Bybit will not open an account. That is an account check, not an API feature.

## Candidates

| Provider | Artificial funds | Algorithmic API | Prices | Practical limits |
|---|---|---|---|---|
| Binance Spot Demo Mode | Yes, reset in the UI | Same spot REST, websocket, and FIX paths as production, on `demo-api.binance.com` | Close to the live book | Needs a Binance login. `/sapi` wallet endpoints are not the point of this mode. [Docs](https://developers.binance.com/docs/binance-spot-api-docs/demo-mode/general-info) |
| Binance Spot Testnet | Yes, granted on GitHub login | Spot `/api` only, on `testnet.binance.vision` | Independent of the live book | Resets about once a month, including orders. No `/sapi`. [Testnet](https://testnet.binance.vision/), [FAQ](https://www.binance.info/en/support/faq/detail/ab78f9a1b8824cf0a106b4229c76496d) |
| Bybit Demo Trading | Yes, demo funds, no deposit for the demo account | v5 REST on `api-demo.bybit.com`; private websocket on `stream-demo.bybit.com` | Public market data is the mainnet stream | Not every endpoint exists. Demo orders are kept 7 days. The key must be created inside Demo Trading, not on testnet. [Docs](https://bybit-exchange.github.io/docs/v5/demo) |
| Kraken Futures demo | Was a separate paper environment | REST and websocket matched production | Separate demo site | The published demo host `demo-futures.kraken.com` was scheduled to be turned off on 14 July 2026. Do not build on it. [Kraken note](https://support.kraken.com/articles/360024809011-api-testing-environment-derivatives) |

## Why the others wait

- **Spot Testnet** is free and easy (GitHub login, no exchange account), and its book is not the live market. It is the right place to test a new Binance feature, and the wrong place to judge a strategy.
- **Bybit** is the fallback when Binance is unavailable. It is a second integration, not a second source of truth.
- **Kraken's old futures demo** is not a current option.
