---
title: MACD
slug: macd
kind: indicator
unit: price
status: implemented
algorithm: v1
series:
  - key: macd-histogram-3h
    definition:
      type: Macd
      method: histogram
      fast_time_frame_hr: 12
      slow_time_frame_hr: 24
      signal_time_frame_hr: 9
inputs:
  - Aggregation closes (3h bar clock)
---

## Definition

MACD **histogram** on the decision bar: EMA(12h) − EMA(24h) minus signal EMA(9h) of the MACD line.
