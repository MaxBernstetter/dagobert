---
title: Exponential moving average
slug: exponential-moving-average
kind: indicator
unit: price
status: implemented
algorithm: v1
series:
  - key: ema-3h
    definition:
      type: MovingAverage
      method: ema
      time_frame_hr: 3
inputs:
  - Aggregated bar closes from the process job
---

## Definition

The **exponential moving average (EMA)** weights recent closes more heavily. `time_frame_hr` is the lookback in hours; period $n$ is that window in bars ($n = \texttt{time\_frame\_hr} / \text{bar duration in hours}$).

## Formula

$$
\text{EMA}_t = \alpha \cdot C_t + (1 - \alpha) \cdot \text{EMA}_{t-1}, \qquad \alpha = \frac{2}{n+1}
$$

When $n = 1$, $\alpha = 1$ so EMA equals the current close.
