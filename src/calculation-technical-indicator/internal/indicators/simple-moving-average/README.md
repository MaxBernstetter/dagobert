---
title: Simple moving average
slug: simple-moving-average
kind: indicator
unit: price
status: implemented
algorithm: v1
series:
  - key: sma-3h
    definition:
      type: MovingAverage
      method: sma
      time_frame_hr: 3
  - key: sma-6h
    definition:
      type: MovingAverage
      method: sma
      time_frame_hr: 6
  - key: sma-12h
    definition:
      type: MovingAverage
      method: sma
      time_frame_hr: 12
  - key: sma-24h
    definition:
      type: MovingAverage
      method: sma
      time_frame_hr: 24
  - key: sma-48h
    definition:
      type: MovingAverage
      method: sma
      time_frame_hr: 48
inputs:
  - Aggregated bar closes from the process job
---

## Definition

The **simple moving average (SMA)** is the unweighted mean of the last $n$ bar closes. `time_frame_hr` is the lookback in hours; $n$ is that window expressed in bars ($n = \texttt{time\_frame\_hr} / \text{bar duration in hours}$).

## Formula

$$
\text{SMA}_t = \frac{1}{n} \sum_{i=0}^{n-1} C_{t-i}
$$

With $n = 1$ (`sma-3h` on v1), $\text{SMA}_t = C_t$.

## Used by

Playbook `ma-cross` uses series `sma-3h`.
