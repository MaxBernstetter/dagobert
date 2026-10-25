# Technical indicator parquet cache (grooming)

## Problem

Experiments recompute 3h indicators from candle parquet on every run (`BuildKPIRange`). Live TI persists JSON bodies in `technical_indicator.sqlite`. Neither is a language-agnostic, reload-cheap lake object.

## Target layout

```text
data/parquet/ti/provider={id}/base={B}/quote={Q}/algo={algo}/year={YYYY}/bars.parquet
```

### Row schema (v1)

| Column | Type | Notes |
|--------|------|--------|
| `bar_open` | int64 | UTC ms, aggregated bar open |
| `open` / `high` / `low` / `close` / `volume` | float64 | Aggregated OHLCV |
| `close_time` | int64 | UTC ms |
| `results_json` | string | Temporary: indicator graph JSON array (same as KPI body `results`) |

**Follow-up (v2):** explode indicators into typed columns (or nested Arrow lists) and drop `results_json`.

## Cache semantics

- **Write:** after a successful `BuildKPIRange` (or live range job), upsert year partitions covering produced bars.
- **Read (warm):** if every year spanning `[from, to)` has a TI partition and min/max `bar_open` covers the requested aggregated range, load rows and skip candle re-aggregate.
- **Cold:** missing/partial coverage → compute from candle lake → write TI parquet → return rows.

## Non-goals

- Replacing in-memory `kpi.Window` during instance sim (still RAM).
- Changing live per-candle job topology in the first cut.
