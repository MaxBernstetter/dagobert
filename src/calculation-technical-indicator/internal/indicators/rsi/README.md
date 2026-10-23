---
title: Relative strength index
slug: rsi
kind: indicator
unit: index
status: implemented
algorithm: v1
series:
  - key: rsi-3h
    definition:
      type: Rsi
      time_frame_hr: 3
inputs:
  - Segment closes of the active aggregated bar (`SegmentClose`)
---

## Definition

**RSI** measures momentum from price changes within the active bar. `time_frame_hr` must align with a valid bar window for the deployment; the formula uses consecutive segment closes inside that bar.

## Formula

For segment closes $c_0,\ldots,c_{k-1}$, let $\Delta_i = c_i - c_{i-1}$ for $i \ge 1$. Average gain $\bar{G}$ and loss $\bar{L}$ are means of positive and absolute negative $\Delta_i$.

$$
\text{RS} = \frac{\bar{G}}{\bar{L}}, \qquad
\text{RSI} = 100 - \frac{100}{1 + \text{RS}}
$$

Edge cases: if $\bar{G} = \bar{L} = 0$, RSI $= 50$; if $\bar{L} = 0$ and $\bar{G} > 0$, RSI $= 100$.
