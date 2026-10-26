---
title: Sharpe ratio
slug: sharpe
kind: strategy-kpi
label: Sharpe
unit: ratio
format: decimal
order: 3
---

## Definition

Risk-adjusted return using periodic portfolio returns on each 3h bar close in range. Annualized with $\sqrt{N}$ where $N$ is 3h periods per year. Risk-free rate is 0 unless configured later.

## Formula

$$
\text{Sharpe} = \frac{\bar{r}}{\sigma_r}\sqrt{N}
$$

with $\bar{r}$ the mean simple return between consecutive equity samples and $\sigma_r$ their standard deviation.
