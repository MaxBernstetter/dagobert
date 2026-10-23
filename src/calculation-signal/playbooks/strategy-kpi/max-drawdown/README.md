---
title: Maximum drawdown
slug: max-drawdown
kind: strategy-kpi
label: Max DD
unit: ratio
format: percent
---

## Definition

Largest peak-to-trough decline in mark-to-market portfolio equity within the range. `recovery_days` (when present) is calendar days from the trough back to the prior peak level; omitted if still underwater at range end.

## Formula

$$
\text{MDD} = \max_t \frac{\max_{s \le t} V_s - V_t}{\max_{s \le t} V_s}
$$
