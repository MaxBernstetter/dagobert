---
title: ATR
slug: atr
kind: indicator
unit: price
status: implemented
algorithm: v1
series:
  - key: atr-12h
    definition:
      type: Atr
      method: wilder
      time_frame_hr: 12
  - key: atr-24h
    definition:
      type: Atr
      method: wilder
      time_frame_hr: 24
inputs:
  - Aggregation high, low, and prior close
---

## Definition

**Average true range** (Wilder) over the lookback window in hours.
