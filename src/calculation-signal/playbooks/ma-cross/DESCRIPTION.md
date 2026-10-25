---
title: Moving-Average Cross
slug: ma-cross
kind: playbook
status: active
symbols:
  - BTCUSDT
  - ETHUSDT
venue: bybit
price_source: binance
---

## Intent

**Paper-trading** playbook: **golden cross** / **death cross** between SMA(3h) and SMA(6h). Long-only. Layer 1 emits **signals** only; sizing is `calculation-market-action`.

## Stateless evaluation

| Input | Source |
|-------|--------|
| SMA this / prior bar | `technical_indicator.sqlite` |
| `flat` / `long` | Prior `signal` rows for this playbook + symbol |

Stored in `calculation_signal.sqlite` → `signal` (not `action`).

## Rules

| Rule | Condition | Signal |
|------|-----------|--------|
| `golden_cross` | `flat`, cross up | `buy` |
| `death_cross` | `long`, cross down | `sell` |
| `none` | Otherwise | `no-trade` |

## Position sizing

The buy and the sell that go with a sized trade are specified in [Position sizing](/wiki/playbooks/position-sizing). This playbook applies sizing when configured: with a `sizing` block in the strategy config, the golden cross buys a risk-sized notional with an ATR(24h) stop, and no ATR means no trade. While long, one bar does one thing in this order: `atr_stop` (close at or below the highest open stop, sell all), `death_cross` (sell all), `vol_cut` (partial sell), `pyramid` (buy, only when `maxUnits` is above 1). Without `sizing` it emits the cross signal only and a run sizes the order as one target weight of the book.

## Sparse lake / gaps

Incomplete 3h aggregations (missing source 1h candles) are omitted from the evaluate timeline. This playbook does **not** special-case `GapFromPrevious`; position is held across skipped bars until the next golden/death cross on an available bar.
