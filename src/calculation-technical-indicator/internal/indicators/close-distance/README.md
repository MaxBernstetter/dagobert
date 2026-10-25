---
title: Close distance
slug: close-distance
kind: indicator
unit: ratio
status: implemented
algorithm: v1
series:
  - key: close-distance-sma-24h
    definition:
      type: CloseDistance
      method: sma
      time_frame_hr: 24
inputs:
  - Bar close and a moving average with the same `time_frame_hr`
---

## Definition

**Close distance** is the relative gap between the current close and a moving average over `time_frame_hr`. The deployed window is 24h: a window of one bar (3h) would average only the current close, so the value would always be 0.

## Formula

$$
d_t = \frac{C_t - \text{MA}_t}{\text{MA}_t}
$$

Omitted when $\text{MA}_t = 0$.
