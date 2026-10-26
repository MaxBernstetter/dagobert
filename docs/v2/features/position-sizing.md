---
title: Position sizing
slug: position-sizing
kind: method
status: specified
---

# Position sizing

Status: specified. ma-cross and signal-consensus apply this when `sizing` is configured. A buy still deploys one target weight of the book, and a sell still exits the whole position. The C++ `SmaCrossSignal`/`SmaCrossStrategy` run the same flow when given a `SizingConfig` (checked against recorded Go output in `sma_cross_sizing_test.cpp`). Both strategy executables (`strategy-sma-cross`, `strategy-signal-consensus`) read the optional `sizing` block of their payload (proto `Sizing`, every field optional); an invalid value exits 2, and without it the run is unchanged. With `sizing` set, `RunReport.sizing` (proto `ActionSizing`, `schema_version` 2) carries one entry per action: notional, stop price, requested and capped risk percent (of equity) and the rule id; sells carry notional and rule only. Without `sizing` the list is empty and the run differs only by the bumped `schema_version`.

## Aim

Raise trading return by making the buy and the sell agree on how much equity can be lost.

The size of a trade is the fraction of equity lost if the stop is hit. Notional weight is the result of that fraction and the stop distance. A wide stop buys less coin. A tight stop buys more, up to the whole book, because the account is spot and unlevered.

Return is judged later, on the same symbol and range as the current rules, with the experiment figures already in use: total return, active return against buy-and-hold, maximum drawdown, and Sharpe. The path to a higher return is a smaller loss when the stop hits, and a winner that is held until the stop or the playbook's trend exit. This page does not claim a backtest result.

## Buy

Buy when the playbook's entry is true and a stop price exists. Size that buy so a fill of the stop loses `unitRiskPct` of equity at entry. Skip the buy when the resulting order is below `minOrderQuote`.

A later rise in the mark does not increase the open trade. The next entry is sized from equity at that entry.

A further buy, only when `maxUnits` is above 1, requires all of the following:

- Price has risen at least `addAtrMult` times ATR(24h) from the last fill. The default multiple is 0.5.
- Open units are below `maxUnits`.
- The new unit, sized from current equity and current ATR, plus the entry-to-stop loss still open on earlier units, stays within `heatPct`.
- Every earlier stop moves up by that same ATR step. A stop never moves down.

With `maxUnits` at 1, which is the default, there is no second buy.

## Sell

Sell the whole position when the stop trades, or when the playbook's existing exit fires. One bar does one thing, in this order: the full exit, then a volatility cut, then a pyramid buy.

The volatility cut is a partial sell. If current ATR implies the open notional would risk more than `heatPct` at `stopAtrMult`, sell the excess base so implied risk is back within the cap. The stop price stays where it is.

A pullback that has not reached the stop leaves the size unchanged.

## Size

ATR is the 24-hour average true range, in price units. The stop distance is `stopAtrMult` times that ATR. The default multiple is 2.

$$
\text{notional} = \min\left(\frac{\text{equity} \times \text{unitRiskPct} / 100}{\text{stopAtrMult} \times \text{ATR} / \text{fill}},\ \text{equity} \times \text{maxWeightPct} / 100\right)
$$

Base size is notional divided by the fill price. When the weight cap binds, the unit risks less than `unitRiskPct`. Record both the requested risk and the risk after the cap.

## Options

These fields sit under `sizing` in the strategy config. Omitting `sizing` keeps today's single target weight and full exit, so an existing experiment stays comparable until it opts in.

| Option | Default | Allowed | Meaning |
|---|---|---|---|
| `unitRiskPct` | 1 | 0.25 to 2 | Loss at the stop for one unit, percent of equity at entry |
| `heatPct` | 2 | up to 2, and at least `unitRiskPct` | Cap on the summed entry-to-stop loss of open units |
| `maxUnits` | 1 | 1 to 4 | Buys in one trade, including the first |
| `stopAtrMult` | 2 | positive | Stop distance in ATR(24h) |
| `addAtrMult` | 0.5 | positive | Favourable move required before another unit |
| `maxWeightPct` | 100 | up to 100 | Notional cap, percent of equity |
| `minOrderQuote` | 1 | 0 or more | Smallest sized order in quote; 0 disables the strategy minimum |

A sized buy, a pyramid buy or a `vol_cut` partial sell worth less than `minOrderQuote` is skipped (the bar does nothing). Full exits (`atr_stop`, cross sells) always run. The playbooks apply this in the signal job and in the backtest, the same way as the C++ SDK. The venue minimum (`MinOrderQuote` in market-action) stays a separate, independent floor on the actual order; cash reserve is a venue limit too.

`unitRiskPct` of 1 and `maxUnits` of 1 is one entry, one percent of equity to the stop, and no pyramid. Four units at 0.5% fit under the 2% heat cap. The 2% cap is the ceiling for the whole position, because one instance holds one symbol.

## Playbooks

**ma-cross.** Buy on the golden cross, at the notional above, with the stop at the fill minus `stopAtrMult` times ATR(24h). Sell all when that stop trades or on the death cross, whichever comes first. The death cross remains the trend exit. The ATR stop is the exit for a loser, which a cross does not provide. The playbook does this when `sizing` is configured.

**signal-consensus.** The entry filters stay: SMA12 cross, the higher trend stack, MACD histogram, RSI and volume votes, and the cooldown. The buy uses the existing ATR stop distance, `atrStopMult`, as `stopAtrMult`, so the size matches the sell. Sell all on the cross below SMA12 or on that ATR stop. The volatility cut is the only new exit. The playbook sizes the buy this way when `sizing` is set.

## Example

Book of 100 000. Bitcoin at 100 000. ATR(24h) is 2 000, so 2% of price. `stopAtrMult` is 2, so the stop is 4% away. `unitRiskPct` is 1 and `maxUnits` is 1.

Risk money is 1 000. Notional is 1 000 / 0.04 = 25 000, which is 0.25 bitcoin, a quarter of the book. The stop is 96 000.

At an ATR of 5% of price and the same multiple, the stop is 10% away and the notional is 10 000. The position is smaller because the stop is wider.

At an ATR of 0.4% of price and the same multiple, the formula wants more than the book. Notional caps at 100 000, and the unit risks 0.8% rather than 1%.

## Left out

Full Kelly, and optimal f, as the size. Adding because the mark rose. Selling because price gave back ground the stop has not reached. Leverage. One heat budget shared across symbols.

## Literature

The fraction and the stop distance are Elder's fixed fraction. In *Come Into My Trading Room* (Wiley, 2002) the risk on one trade is at most 2% of equity, and the position is that money divided by the entry-to-stop distance. The same book's 6% rule stops new risk for the month once month-to-date losses plus open risk reach 6%. That rule is for several concurrent risks. A single instance already refuses an entry that would push this trade through `heatPct`.

Van Tharp, *Trade Your Way to Financial Freedom*, states the same identity in R-multiples. The entry-to-stop loss is 1R. Bet size is what maps a distribution of R onto an equity curve, and the next trade is sized from equity at entry.

The Original Turtle Trading Rules (Richard Dennis and William Eckhardt) size a unit from volatility. N is a 20-day average true range. One unit is sized so a 1N move equals 1% of equity. A unit is added only after a favourable move of half N, at most four units in one market, with a stop at 2N. When a unit is added, earlier stops move up by half N. That per-unit budget assumed a book of many futures markets. Here every unit shares the 2% heat cap. The method is the one above: add only into a trade that is already working, trail the stop up, and cap the count. The circulated text is [The Original Turtle Trading Rules](https://tradingwithrayner.com/wp-content/uploads/2014/11/OriginalTurtleRules.pdf).

Moreira and Muir, "Volatility-Managed Portfolios", *Journal of Finance* 72 (4), 2017, scale exposure by the inverse of recent realized variance, because variance persists and expected return does not rise in proportion. [doi.org/10.1111/jofi.12513](https://doi.org/10.1111/jofi.12513). The volatility cut is that direction applied as a cap: less notional when ATR is higher. A quieter ATR is not, by itself, a reason to add.

Kelly, "A New Interpretation of Information Rate", *Bell System Technical Journal* 35 (4), 1956, gives the fraction that maximizes expected log growth. [doi.org/10.1002/j.1538-7305.1956.tb03809.x](https://doi.org/10.1002/j.1538-7305.1956.tb03809.x). Full Kelly, and the related optimal f, produce deep drawdowns and need an edge estimate these playbooks do not have yet. Rotando and Thorp, "The Kelly Criterion and the Stock Market", *American Mathematical Monthly* 99 (1992), apply that fraction to a market edge. [PDF](https://www.edwardothorp.com/wp-content/uploads/2016/11/TheKellyCriterionAndTheStockMarket.pdf). A measured half of the Kelly fraction can later be compared with `unitRiskPct`. It does not set the fraction, and it stays under the 2% ceiling. In the continuous approximation, that half keeps about three-quarters of the growth rate.

## Implementation notes

The pure maths lives in `src/calculation-signal/internal/sizing`; `position-sizing-vectors.json` holds the worked examples as golden vectors. Readings where this page was silent:

- Open risk of earlier units is `qty x (fill - stop)`, computed after their stops are raised by the ATR step.
- The volatility cut sells `(base x stopAtrMult x ATR - equity x heatPct/100) / (stopAtrMult x ATR)`, capped at the base held.
- Equity is cash plus base at the bar mark. A decision may carry `QuoteNotional`, `BaseQty` and `StopPrice`; market-action caps a notional by cash above the reserve and skips a buy below the venue minimum.
- The backtest loop tracks open units from simulated fills. The live job reads `STRATEGY_CONFIG` (playbook JSON with `sizing`; unset keeps the legacy signal) and rebuilds open units from the fill journal and the stored signal at each fill (`stop_price`, `raise_stops_by`, `base_qty`), with the same rules: stops only rise, a partial sell scales every unit, a full exit or a remainder below the venue minimum clears them. A malformed config exits with detail 41 and writes no signal.
- Live equity is the `EQUITY` env value (quote units) when set, else the paper wallet marked at the bar close, else 0, which skips a sized entry. The signal body carries `quote_notional`, `base_qty`, `stop_price` and `raise_stops_by` only for sized decisions, and market-action passes the first two to its allocator. Live Bybit equity is not read by the signal job (it holds no credentials).
- The C++ SDK applies the signal-consensus rules in `ConsensusSignal` (`src/calculation/sdk/signals`) when a `SizingConfig` is passed, with `stopAtrMult` set to the configured `atrStopMult` (`ConsensusParams`, default 2.5). Per bar: `atr_stop` (the fill-based stop, outside the min-bars-long gate), `consensus_sell`, `vol_cut`, `pyramid`. Without a config it keeps `buy_all` and `sell_all`. `tests/fixtures/consensus_sizing.json` holds Go-generated expected fills (regenerate with `CONSENSUS_FIXTURE_WRITE=1`, see `cpp_fixture_test.go`). The C++ and Go `SizingConfig` both have an optional `minOrderQuote` (default 1.0 quote, validated >= 0): a sized buy, pyramid buy or vol_cut partial sell worth less is skipped, while full exits always run. A non-positive stop price does not block a buy (it never triggers), as in Go. The Go-generated fixtures run with the default minimum. The proto `Sizing` message carries `minOrderQuote` (default 1.0 when absent).
