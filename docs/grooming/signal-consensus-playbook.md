# Playbook: signal-consensus (multi-indicator voting)

## Goal

Add a **second playbook** that is more realistic than `ma-cross`: several **independent-ish** measurements, each emitting a **sub-signal**, combined into **buy | hold | sell** while keeping today’s execution model:

- **Buy:** fixed quote allocation (market-action / `ALLOC_*`, unchanged).
- **Sell:** close **entire** long position (unchanged).
- **Hold:** `no-trade` while `long` (same as today).

Signal layer only; no sizing logic in the playbook.

---

## Research: what to combine?

### Principle

Use indicators that answer **different questions**, not multiple views of the same trend:

| Question | Family | Avoid pairing with |
|----------|--------|-------------------|
| Which way is the market drifting? | Trend / level vs baseline | Another MA cross (same as `ma-cross`) |
| Is upside pressure building or fading? | Momentum oscillator | Second oscillator on same closes (e.g. RSI + Stoch) |
| Is activity confirming the move? | Volume / participation | Raw volume + volume MA without ratio |
| Is price dangerously stretched? | Mean-reversion **risk** filter | Same as trend vote (e.g. “above MA” twice) |

Literature and practice: **confirmation** (Murphy; multi-condition screens) and **factor intuition** — trend and short-horizon momentum are related but not identical; **volume** often adds information not captured by price-only rules (e.g. volume confirmation in breakout systems; “volume as conviction” in many systematic TA designs).

We **do not** need ML for v1: a **score / quorum** over interpretable rules is enough and matches the current `framework.Decision` model.

### Why not another MA cross?

`ma-cross` already trades **events** on SMA(3h) vs SMA(6h). A new playbook should not duplicate that. Use:

- **EMA level** or **close vs slow SMA** for regime (slow, different trigger).
- **RSI** for momentum (already in TI v1).
- **Volume vs its mean** for participation (already in TI v1).
- **Close distance** as a **stretch guard**, not a fourth directional vote (high correlation with “close above MA”).

---

## Proposed strategy: **signal-consensus**

**Style:** Long-only spot, 3h decision cadence (same bar clock as live stack).  
**Slug:** `signal-consensus`  
**Logic version:** `1` (bump when rules change).

### Indicators used (all already in `Deployments()` v1)

| Key | Type | Role |
|-----|------|------|
| `ema-3h` | EMA 3h | Trend regime |
| `sma-6h` | SMA 6h | Slow baseline for stretch |
| `rsi-3h` | RSI 3h | Momentum |
| `volume-mean-3h` | Mean volume 3h | Participation baseline |
| `close-distance-sma-3h` | Close vs SMA 3h | Stretch / blow-off guard |

No new TI deployments required for v1.  
**Gap:** strategy needs **current bar volume** to form `volume_ratio = V_t / volume_mean_3h`. Today KPI stores the mean, not the ratio. Options:

1. **v1:** Add `kpi.BarVolume(results)` from the KPI OHLC block if bar volume is stored on the revision body (preferred, small `kpi` helper).
2. **v1 fallback:** Skip volume vote until bar volume is available; use **2-of-3** quorum (trend + momentum + stretch) and add volume in v1.1.

### Sub-signals (each bar)

Evaluated on `ctx.Results` / `ctx.PreviousResults` (same as `ma-cross`).

**1 — Trend (`trend`)** — *directional*

- **Bull:** `close > EMA(3h)`
- **Bear:** `close < EMA(3h)`
- **Neutral:** else (rare)

Uses EMA, not SMA cross, so measurement differs from `ma-cross`.

**2 — Momentum (`momentum`)** — *directional*

- **Bull:** `RSI(3h) ≥ 52` **or** (`RSI > 50` and `RSI > RSI_prev`)
- **Bear:** `RSI(3h) ≤ 48` **or** (`RSI < 50` and `RSI < RSI_prev`)
- **Neutral:** otherwise

Thresholds avoid chop around 50; slope breaks ties.

**3 — Participation (`volume`)** — *directional when ratio available*

- `ratio = bar_volume / volume_mean(3h)`
- **Bull:** `ratio ≥ 1.10`
- **Bear:** `ratio ≤ 0.85`
- **Neutral:** else

**4 — Stretch guard (`stretch`)** — *not a vote; veto / exit helper*

- `d = close_distance_sma(3h)` (fraction, e.g. `0.03` = +3%)
- **Block new buy** if `d > +0.05` (overextended) or `d < -0.04` (falling knife)
- **Contribute to sell** when long if `d > +0.06` or `d < -0.05`

### Aggregate decision (long-only)

Count **bull** and **bear** among `{trend, momentum, volume}` (max 3).

| Position | Action | Condition | `RuleID` (example) |
|----------|--------|-----------|-------------------|
| `flat` | **buy** | `bull ≥ 2` and `bear == 0` and stretch allows entry | `consensus_buy` |
| `flat` | hold | else | `none` |
| `long` | **sell** | `bear ≥ 2` **or** stretch exit **or** (`bear ≥ 1` and `bull == 0`) | `consensus_sell` / `stretch_exit` |
| `long` | hold | else | `none` |

`no-trade` while long = **hold** (unchanged contract for downstream).

Quorum **2-of-3** is a common compromise: strict enough to filter noise, looser than requiring unanimity (which would rarely trade on 3h crypto).

### Indicators **not** in TI v1 (need new deployments before use)

| Idea | Why | Notes |
|------|-----|--------|
| ATR / volatility stop | Size-aware exits | New indicator + playbook param |
| MACD / PPO | Slower momentum | Avoid duplicating RSI without research |
| SMA 12h–24h | Slower regime on 3h bars | Add `sma-12h` etc. to `Deployments()` |
| Funding / sentiment | Crypto-specific | Out of scope for lake backtest |

v7 playbook uses **only** existing v1 KPIs; tune thresholds in `strategy.go` before adding formulas.

### Parameters (future YAML; constants in v1)

| Param | Default | Notes |
|-------|---------|-------|
| `rsi_bull` | 52 | |
| `rsi_bear` | 48 | |
| `vol_ratio_bull` | 1.10 | |
| `vol_ratio_bear` | 0.85 | |
| `buy_quorum` | 2 | |
| `sell_quorum` | 2 | |
| `stretch_buy_max` | +0.05 | |
| `stretch_buy_min` | -0.04 | |

---

## Correlation sanity check

| Pair | Expectation |
|------|-------------|
| Trend (close vs EMA) vs Momentum (RSI) | Moderate; RSI uses intra-bar segment deltas |
| Trend vs Volume ratio | Lower; volume is not price level |
| Trend vs Stretch guard | Correlated; guard is **one-sided filter**, not second trend vote |
| ma-cross vs signal-consensus | Different triggers (event cross vs quorum); may trade same bars sometimes — acceptable for A/B backtests |

---

## Implementation plan (stacked, reviewable PRs)

1. **`kpi` readers** — `RSI`, `CloseDistance`, `VolumeMean`, optional `BarVolume` from KPI body (mirror `MovingAverage`).
2. **`framework.Context` helpers** — thin wrappers calling `kpi.*`.
3. **Playbook package** — `src/calculation-signal/playbooks/signal-consensus/` (`strategy.go`, `strategy_test.go`, `DESCRIPTION.md`); register in `playbooks/registry.go`.
4. **Wiki / backtest catalog** — expose playbook in UI pickers (if not already generic).
5. **System test** — extend `tests/system/backtest` or signal fixture with seeded KPI rows covering buy/sell/hold paths (no runtime SLO).

Keep `ma-cross` unchanged for regression comparison.

---

## Backtest / research workflow

1. Run same symbol/range on `ma-cross` vs `signal-consensus` in Backtesting UI.
2. Compare decisions count, fill count, drawdown (existing KPI tags).
3. Tune quorum/thresholds only after v1 is wired (separate issue; avoid tuning in first PR).

---

## References (conceptual)

- J. Murphy, *Technical Analysis of the Financial Markets* — confirmation across indicator types.
- Elder “triple screen” — inspiration for separating trend and entry timing (here collapsed to one bar with multiple scores).
- Practitioner “scoring” / checklist systems (trend + momentum + volume) widely used in discretionary and systematic TA; quorum is the discrete version.
