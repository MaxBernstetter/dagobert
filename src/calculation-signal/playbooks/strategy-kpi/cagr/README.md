---
title: CAGR (net)
slug: cagr
kind: strategy-kpi
label: CAGR
unit: ratio
format: percent
---

## Definition

Compound annual growth rate of mark-to-market portfolio equity over the evaluation window, from the first to the last sample in range. Assumes fills replayed at stored prices; no extra fee model beyond fill data.

## Formula

$$
\text{CAGR} = \left(\frac{V_{\text{end}}}{V_{\text{start}}}\right)^{1/T_{\text{years}}} - 1
$$

where $V$ is portfolio equity and $T_{\text{years}}$ is the elapsed calendar time between range endpoints.
