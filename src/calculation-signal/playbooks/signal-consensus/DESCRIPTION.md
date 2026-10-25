---
title: Signal consensus
slug: signal-consensus
kind: playbook
status: active
symbols:
  - BTCUSDT
  - ETHUSDT
venue: bybit
price_source: binance
---

## Intent

**Long-only** on **6h bars** with a **higher-level trend stack** (close above SMA24 & SMA48, SMA24 > SMA48), **SMA12 cross** + **MACD histogram > 0** entry, **ATR** stop below SMA12.

## Features (TI v1)

| Feature | Series | Use |
|---------|--------|-----|
| SMA 12h / 24h / 48h | `sma-12h`, `sma-24h`, `sma-48h` | Cross trigger, regime, macro trend |
| MACD histogram | `macd-histogram-3h` | Momentum confirmation |
| ATR 24h | `atr-24h` | Volatility stop |
| RSI, volume | `rsi-6h`, volume mean 6h | Filters |

Backtests set `AGGREGATION_DURATION_MS=6h` for this playbook.

## Rules (v9)

| Rule | Condition | Signal |
|------|-----------|--------|
| `consensus_buy` | flat, cross up SMA12, higher trend stack, MACD hist > 0, filters, cooldown | `buy` |
| `consensus_sell` | long, cross below SMA12 | `sell` |
| `atr_stop` | long, close < SMA12 − 2.5×ATR24 | `sell` |

## Position sizing

The size that matches this stop is specified in [Position sizing](/wiki/playbooks/position-sizing). This playbook does not apply that spec yet. It still emits the signal only, and a run still sizes the order as one target weight of the book.
