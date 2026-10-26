---
title: Win rate
slug: win-rate
kind: strategy-kpi
label: Win rate
unit: ratio
format: percent
order: 6
---

## Definition

Share of closed trades that made money. A closed trade is one sell fill. The sell wins when realized quote, marked at average cost from fills inside the window, is above zero. A sell that realizes exactly zero is a breakeven: it counts in the denominator and in neither gain nor loss. A sell with no inventory is skipped. Fees stay out of the mark.

Month and year figures are the mean and median of UTC calendar buckets. A bucket with no closed trades is omitted. The experiment figure sums wins and closed trades across completed instances; it is not the average of instance win rates.

## Formula

$$
\text{Win rate} = \frac{\text{wins}}{\text{wins} + \text{losses} + \text{breakevens}}
$$

Gain and loss magnitudes are the positive quote of each non-zero sell, and that quote divided by the cost of the base sold. A zero cost keeps the quote sample and drops the percent sample. The ladder reports the average, median, and the 25%, 10%, and 5% of trades at or above that magnitude.
