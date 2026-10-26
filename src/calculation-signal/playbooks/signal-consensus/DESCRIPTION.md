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

### Series as computed (decision per window)

The playbook reads the technical-indicator job's series, and those are not the textbook indicators. Their behaviour is the specification for now and the C++ executable reproduces it (checked on 700+ bars, see `cpp_equivalence_fixture_test.go`). Each row is a known property, not a validated design choice; changing one is a separate decision for the owner.

| Series | Definition on 3h bars | Ready when |
|--------|-----------------------|------------|
| bars | Candles of the lake (1h) grouped at UTC multiples of 3h; a bar exists only when all its source candles exist, so a missing hour drops the bar and the next bar follows it in every window. Volume is the sum, close the last candle's close. | |
| `sma-Nh` | Mean of the last N/3 bar closes (4, 8, 16 for 12h, 24h, 48h). | N/3 bars |
| `atr-24h` | Mean of the last 8 true ranges (true range uses the previous bar's close). The Wilder loop seeds on the last 8 and has nothing left to smooth, so it is a plain mean. | 9 bars |
| `rsi-3h` | Mean gain over mean loss of the three 1h closes inside the current 3h bar (two deltas); 50 when flat, 100 without losses. It does not look at earlier bars. | 2 or more source closes (never on a 3h lake) |
| `volume-mean-3h` | Mean of the last one bar volume, i.e. the bar's own volume. The ratio is exactly 1, so the volume vote is neutral unless `volRatioBear` >= 1 (bear, entries blocked) or `volRatioBull` <= 1 (bull). | 1 bar |
| `macd-histogram-3h` | 12h/24h/9h = 4/8/3 bars. Fast and slow EMAs each seed on only the last 4 (8) closes; the signal EMA seeds on the last 3 MACD values; histogram = MACD minus signal. | 11 bars |

KPI rows keep a trailing window of 16 bars (the longest series, 48h), so no series depends on earlier history. A bar after a gap (previous 3h bar missing) is not traded.

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
