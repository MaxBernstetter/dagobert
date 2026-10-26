---
title: Active return vs benchmark
slug: active-return
kind: strategy-kpi
label: vs B&H
unit: ratio
format: percent-pts
order: 4
---

## Definition

Total return of the strategy minus total return of a buy-and-hold benchmark on the same symbol over the same window, using the same starting capital and bar-close marks. Positive means the playbook beat hold.

## Formula

$$
\text{Active} = \frac{V_{\text{end}}}{V_{\text{start}}} - \frac{B_{\text{end}}}{B_{\text{start}}}
$$

where $B$ is buy-and-hold equity.
