---
title: Close distance
slug: close-distance
kind: indicator
unit: ratio
status: implemented
algorithm: v1
series:
  - key: close-distance-sma-3h
    definition:
      type: CloseDistance
      method: sma
      time_frame_hr: 3
inputs:
  - Bar close and a moving average with the same `time_frame_hr`
---

## Definition

**Close distance** is the relative gap between the current close and a moving average of the same `time_frame_hr`.

## Formula

$$
d_t = \frac{C_t - \text{MA}_t}{\text{MA}_t}
$$

Omitted when $\text{MA}_t = 0$.
