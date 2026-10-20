# Research: data input

Decision for the first market-data feed. Checked on 2026-10-20.

## Recommendation

Use the **CoinGecko Demo API** (free key) for reference prices, market cap, and daily or hourly history of common assets. Use **Binance public spot market data** (no key) when the copilot needs exchange candles or a live book for symbols Binance lists.

CoinGecko is the catalogue: one id space for BTC, ETH, and the rest of a liquid list, without tying the product to one exchange. Binance is the tape: deeper candles and a websocket, still free, but only for pairs that venue lists. Do not poll the keyless CoinGecko endpoint on a schedule. It shares an IP pool and is documented as unsuitable for production polling.

## What this does not cover

- A full order book or trades from CoinGecko. Those come from the venue, later, and from Binance only for listed pairs.
- Sub-minute history. Demo historical depth is daily for about a year and hourly for about a year; 5-minute history on that plan is about a day. See [CoinGecko pricing](https://www.coingecko.com/en/api/pricing).
- Guaranteed uptime or a support contract. Both feeds are free tiers.

## Candidates

| Source | Cost | Common assets | History | Limits | Notes |
|---|---|---|---|---|---|
| CoinGecko Demo | Free key, attribution | Broad catalogue, not one venue | Daily and hourly on the free plan; chart endpoint | 10,000 calls/month, 100 calls/minute on the documented Demo plan; data from about 60 seconds | [Pricing](https://www.coingecko.com/en/api/pricing), [rate limits](https://docs.coingecko.com/docs/errors-and-rate-limits) |
| CoinGecko keyless | No account | Same public endpoints | Same shape as Demo, tighter quota | About 10–30 calls/minute, shared by IP | [Keyless API](https://docs.coingecko.com/docs/keyless-public-api). Fine for a spike, not for a job. |
| Binance public spot API | No key for market data | Pairs listed on Binance spot, including BTC and ETH | Klines from 1 minute up, plus a websocket | Request weights; public data needs no account | [Market data](https://developers.binance.com/docs/binance-spot-api-docs/rest-api/market-data-endpoints). Misses assets Binance does not list. |
| Coinbase Exchange public API | No key for market data | Coinbase-listed pairs | Candles, trades, book | Public rate limits, no account | [Market data](https://docs.cdp.coinbase.com/exchange/reference/exchangerestapi_getproductcandles). Narrower catalogue than CoinGecko, cleaner than scraping. |

## Why the others wait

- **Keyless CoinGecko** fails the reliability bar for anything on a timer.
- **Binance alone** is the right tape later, and a poor catalogue: delistings and missing names become the product's asset list.
- **Coinbase public data** is a sound second tape if Binance is blocked in the operator's region. It is not a broader catalogue than CoinGecko.
