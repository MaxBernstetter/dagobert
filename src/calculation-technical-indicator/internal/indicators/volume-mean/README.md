---
title: Mean volume
slug: volume-mean
kind: indicator
unit: amount
status: implemented
algorithm: v1
series:
  - key: volume-mean-3h
    definition:
      type: VolumeMean
      time_frame_hr: 3
  - key: volume-mean-6h
    definition:
      type: VolumeMean
      time_frame_hr: 6
  - key: volume-mean-12h
    definition:
      type: VolumeMean
      time_frame_hr: 12
inputs:
  - Aggregated bar volume from the process job
---

## Definition

**Mean volume** is the arithmetic average of bar volume over $n$ completed bars, where $n = \texttt{time\_frame\_hr} / \text{bar duration in hours}$.

## Formula

$$
\bar{V}_t = \frac{1}{n} \sum_{i=0}^{n-1} V_{t-i}
$$
