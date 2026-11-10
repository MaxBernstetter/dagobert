# Wiki catalog for indicators and playbooks

## Goal

**The Wiki markdown is source code.** Wiki content lives only in the files the frontend renders and nowhere else: do not restate it in `docs/`. `src/frontend/src/wikiCatalog.ts` loads them with `import.meta.glob`:

| Glob | Wiki page |
|---|---|
| `src/calculation-technical-indicator/internal/indicators/*/README.md` | indicators (`readme_sync_test.go` checks `series` against the code) |
| `src/calculation-signal/playbooks/*/DESCRIPTION.md` | playbooks and Position Sizing |
| `src/calculation-signal/playbooks/strategy-kpi/*/README.md` | strategy KPIs |

Change a glob and this table together.

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
src/calculation-signal/playbooks/position-sizing/
  DESCRIPTION.md            # the real file (kind: method); position-sizing-vectors.json sits beside it as a test fixture
src/calculation-signal/playbooks/strategy-kpi/<slug>/
  README.md                 # strategy KPI doc (`kind: strategy-kpi` in frontmatter)
```

Formula slugs: `simple-moving-average`, `exponential-moving-average`, `close-distance`, `rsi`, `volume-mean` (one folder per formula). Series keys: `sma-3h`, `ema-3h`, etc.

## DESCRIPTION / README format

Indicator formulas use `README.md` with YAML frontmatter (`slug` = formula, not series key). Playbooks use `DESCRIPTION.md`.

## Formulas in the frontend

Render with **react-markdown**, **remark-gfm**, **remark-math**, **rehype-katex**. See earlier syntax table in git history.

## Wiki routes

| Route | Content |
|-------|---------|
| `/wiki` | Four sections in a 2×2 grid: Technical indicators, Market Action, Position Sizing, Strategy performance indicators. A narrow viewport stacks them in that order. |
| `/wiki/indicators/[formula]` | README + table from `instances/*.json` |
| `/wiki/playbooks/[slug]` | Market Action playbook DESCRIPTION |
| `/wiki/playbooks/position-sizing` | Position Sizing section. Rendered from `src/calculation-signal/playbooks/position-sizing/DESCRIPTION.md` (`kind: method`). |
| `/wiki/strategy-kpi/[slug]` | Strategy KPI README |

The SPA build (`scripts/frontend-deploy.sh`, and the dev image in `src/frontend/Dockerfile`) copies or mounts `src/calculation-technical-indicator` and `src/calculation-signal`, and `task frontend:dev` runs from the repo checkout, so the globs resolve in all of them.

## Quality bar

Go is the source of truth for deployed series (`run.Deployments()`). Each formula implements `Calculate(candles, timeFrameHr)` once; README frontmatter `series` is what the Wiki renders; `TestWikiREADMESeriesMatchCode` fails if it diverges from `Definition(timeFrameHr)` in `indicator.go`.
