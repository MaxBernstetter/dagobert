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

The buy and the sell that go with a sized trade are specified in [Position sizing](/wiki/playbooks/position-sizing). This playbook does not apply that spec yet. It still emits the cross signal only, and a run still sizes the order as one target weight of the book.

