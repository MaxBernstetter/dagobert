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

**Long-only** on **3h bars** with a **higher-level trend stack** (close above SMA24 & SMA48, SMA24 > SMA48), **SMA12 cross** + **MACD histogram > 0** entry, **ATR** stop below SMA12.

## Features (TI v1)

| Feature | Series | Use |
|---------|--------|-----|
| SMA 12h / 24h / 48h | `sma-12h`, `sma-24h`, `sma-48h` | Cross trigger, regime, macro trend |
| MACD histogram | `macd-histogram-3h` | Momentum confirmation |
| ATR 24h | `atr-24h` | Volatility stop |
| RSI, volume | `rsi-3h`, volume mean 3h | Filters |

Uses the default Experiment Suite aggregation (`AGGREGATION_DURATION_MS` unset → 3h).

## Rules (v10)

| Rule | Condition | Signal |
|------|-----------|--------|
| `consensus_buy` | flat, cross up SMA12, higher trend stack, MACD hist > 0, filters, cooldown | `buy` |
| `consensus_sell` | long, cross below SMA12 | `sell` |
| `atr_stop` | long, close < SMA12 − atrStopMult×ATR24 | `sell` |

Thresholds (`rsiBull` / `rsiBear`, volume ratios, ATR stop mult, cooldowns) are runtime `StrategyConfig` for Experiment Suite.

| Config field | Default | Valid range |
|--------------|---------|-------------|
| `rsiBull` | 52 | (0,100], above `rsiBear` |
| `rsiBear` | 48 | [0,100) |
| `volRatioBull` | 1.10 | positive, above `volRatioBear` |
| `volRatioBear` | 0.85 | positive |
| `atrStopMult` | 2.5 | positive |
| `minBarsFlatAfterSell` | 8 | 0 or more |
| `minBarsLongBeforeSell` | 4 | 0 or more |

The C++ executable (`SignalConsensusConfig`) reads the same fields; an unset (zero) proto field takes the default and an invalid value exits 2. The SMA hours (12/24/48) and the ATR window (24h) are fixed.

## Position sizing

Optional `sizing` config applies [Position sizing](/wiki/playbooks/position-sizing). Without it the rules above are unchanged: one target weight in, the whole position out. Rules stay at v10; the config JSON is part of the playbook hash, so sized runs have their own identity.

With `sizing` set, `stopAtrMult` is `atrStopMult`. The entry filters and the cross-below-SMA12 sell are unchanged. The buy is sized so a fill of its stop loses `unitRiskPct` of equity, and the stop is fill-based: fill minus `atrStopMult` x ATR(24h), recorded on the buy with the requested and capped risk. A buy below the venue minimum is skipped.

While long, a bar does one thing, in order:

| Rule | Condition | Signal |
|------|-----------|--------|
| `atr_stop` | close at or below an open unit's stop (fill-based) | sell all |
| `consensus_sell` | cross below SMA12 (after the minimum bars long) | sell all |
| `vol_cut` | ATR implies open risk above `heatPct` | partial sell of the excess |
| `pyramid` | `maxUnits` above 1 and the add rules pass | buy another unit, earlier stops raised |

A long position without tracked units (the live job does not derive them yet) falls back to the SMA12-based `atr_stop` and full exit.
