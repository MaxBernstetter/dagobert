# Wiki catalog for indicators and playbooks

## Goal

Every **indicator formula** and every **playbook** has a folder with documentation beside the code. Aggregation time (`time_frame_hr`) is a parameter of the formula, not a separate indicator type. Deployed calculation keys such as `sma-6h` label a specific series for operators and the Wiki; KPI rows identify values by JSON `def` only.

## Layout

```text
src/calculation-technical-indicator/internal/indicators/interface.go   # TechnicalIndicator, Deployment, Result
src/calculation-technical-indicator/internal/indicators/run/         # Deployments, Results (wires formulas)
src/calculation-technical-indicator/internal/bars/                   # CandleAggregation, AggregationSpec, window helpers
src/calculation-technical-indicator/internal/lake/                   # parquet + interval_ms sidecar metadata
src/calculation-technical-indicator/internal/indicators/<formula>/
  indicator.go              # Definition struct + Indicator implementing TechnicalIndicator
  README.md                 # formula doc; `series` in YAML frontmatter for the Wiki
src/calculation-signal/playbooks/<slug>/
  <slug>.yaml
  DESCRIPTION.md
src/calculation-signal/playbooks/strategy-kpi/<slug>/
  README.md                 # strategy KPI doc (`kind: strategy-kpi` in frontmatter)
```

Formula slugs: `simple-moving-average`, `exponential-moving-average`, `close-distance`, `rsi`, `volume-mean` (one folder per formula). Series keys: `sma-3h`, `ema-3h`, etc.

## DESCRIPTION / README format

Indicator formulas use `README.md` with YAML frontmatter (`slug` = formula, not series key). Playbooks use `DESCRIPTION.md`.

## Formulas in the frontend

Render with **react-markdown**, **remark-gfm**, **remark-math**, **rehype-katex**. See earlier syntax table in git history.

## Runtime paths

| Variable | Default | Docker Compose |
|----------|---------|----------------|
| `WIKI_CATALOG_DIR` | `<repo>/src/calculation-technical-indicator/internal/indicators` | `/wiki/calculation-technical-indicator/internal/indicators` |
| `WIKI_PLAYBOOKS_DIR` | `<repo>/src/calculation-signal/playbooks` | `/wiki/playbooks` |

## Wiki routes

| Route | Content |
|-------|---------|
| `/wiki` | Formulas and playbooks; cards list deployed series keys |
| `/wiki/indicators/[formula]` | README + table from `instances/*.json` |
| `/wiki/playbooks/[slug]` | Playbook DESCRIPTION |
| `/wiki/strategy-kpi/[slug]` | Strategy KPI README |

Strategy KPI markdown is under `playbooks/strategy-kpi/` (same volume as `WIKI_PLAYBOOKS_DIR`). Optional `STRATEGY_KPI_DIR` overrides the catalog path.

## Quality bar

Go is the source of truth for deployed series (`run.Deployments()`). Each formula implements `Calculate(candles, timeFrameHr)` once; README frontmatter `series` is what the Wiki renders; `TestWikiREADMESeriesMatchCode` fails if it diverges from `Definition(timeFrameHr)` in `indicator.go`.
