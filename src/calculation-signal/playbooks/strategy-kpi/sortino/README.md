---
title: Sortino ratio
slug: sortino
kind: strategy-kpi
label: Sortino
unit: ratio
format: decimal
---

## Definition

Like Sharpe, but scales mean return by downside deviation (volatility of negative periodic returns only). Annualized with $\sqrt{N}$ for 3h bars.

## Formula

$$
\text{Sortino} = \frac{\bar{r}}{\sigma_{r^-}}\sqrt{N}
$$

where $\sigma_{r^-}$ uses only returns below zero.
