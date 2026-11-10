# Grooming: scalable experiment / instance API

> Historical: this note describes the removed orchestrator and its REST routes (`/api/v1/...`). The current runtime is in [docs/architecture/local-stack.md](../architecture/local-stack.md); the APIs are Connect services in `proto/`.

## Problem

An experiment was treated as one fat document: create POSTed up to 20k instance specs; list/detail/poll dragged instance rows; the instance table fanned out HTTP/KPI reads from `report.json`. Sort/filter cannot stay near-real-time at that scale.

## Locked decisions

| # | Choice |
|---|--------|
| Create | **Server-owned grid** — client sends settings + grid axes (and/or a small manual instance list); server owns expansion |
| Materialize | **Async** after create (`materializing` → `draft` when instance rows exist) |
| Read model | Experiment = **summary only**; instances = **paginated child collection**; full report = **per-instance on demand** |
| Table KPIs | **Denormalized scalars on the instance row** (written at sim complete); list API sorts/filters in the catalog — **no** N× `report.json` / KPI HTTP for the table |
| Poll | Experiment **progress** endpoint only (no instance array) |
| Storage | **Postgres** (normalized catalog + stats); lake for OHLC/TI — [`serverless-aws.md`](serverless-aws.md) steps 7+. SQLite/DDB are interim only. |
| Cap | Keep **20 000** max valid combos (server-enforced) |

## Resource model

### Experiment (summary)

Never embeds `instances[]`.

```http
GET    /api/v1/experiments
POST   /api/v1/experiments
GET    /api/v1/experiments/{id}
GET    /api/v1/experiments/{id}/progress
POST   /api/v1/experiments/{id}/run
POST   /api/v1/experiments/{id}/cancel
DELETE /api/v1/experiments/{id}
```

`POST /api/v1/experiments` body: `settings` + `mode: "grid"|"manual"` + `playbook` + `grid.axes` **or** small `instances[]`.  
Response: summary with `status: "materializing"`, planned `progressInstanceTotal`.

### Instances (paginated + indexed metrics)

```http
GET /api/v1/experiments/{id}/instances?cursor=&limit=50&status=&symbol=&q=&sort=sharpe&order=desc
GET /api/v1/experiments/{id}/instances/{instanceId}
GET /api/v1/experiments/{id}/instances/{instanceId}/report
GET /api/v1/experiments/{id}/instances/{instanceId}/activity
GET /api/v1/experiments/{id}/instances/{instanceId}/chart-series
GET /api/v1/experiments/{id}/instances/{instanceId}/kpis   # detail only; table must not fan out
```

List row includes lean identity fields **plus denormalized metrics** (nullable until completed):

- `durationMs`
- strategy KPI scalars used as columns: `cagr`, `maxDrawdown`, `sharpe`, `sortino`
- period stats: `monthlyReturnAvg` / `Med`, `yearlyReturnAvg` / `Med`, `tradesPerMonthAvg` / `Med`

Sort/filter happen in SQL on those columns (indexes on `(experiment_id, <metric>)`).

## KPI index write path (Go job)

On instance complete in `run_experiment.go` (alongside `BuildAndWriteInstanceReport`):

1. Extract the fixed scalar set from the in-memory report.
2. `UPDATE experiment_instance SET metric_… = ? … WHERE id = ?`.
3. `report.json` stays the full artifact; catalog holds the list projection only.

Prefer **explicit columns** for every sortable table field so `ORDER BY` stays simple (Postgres indexes on `(experiment_id, metric_*)`).

Backfill from existing `report.json` is a follow-up (not required to ship).

## Materialization

1. Insert experiment (`materializing`), persist `grid_json` (or expand manual specs immediately when small).
2. Async expand via shared grid logic (`experiment-grid.ts` on server), batch-insert instances, update progress.
3. → `draft` or `failed`. UI polls `/progress`.

## UI

- Create posts grid/manual spec only.
- Instance table: **server-driven** page + sort + filter query params.
- Metrics on the row payload; no per-row KPI fan-out for the table.
- Running: poll `/progress`; refresh **current page** of instances.
- Detail: `/report` (and chart/activity) on demand only.

## Out of scope

- Lowering 20k cap
- Keeping `report.json` as the UI source of truth (stats live in Postgres; charts OHLC/TI from lake)
- SSE

Catalog storage implementation: [`serverless-aws.md`](serverless-aws.md) (steps 7+).

## Acceptance

- Create of a max-size grid returns in tens of ms; materialize via `/progress`.
- `GET /experiments` and `/progress` stay fast with 20k instances.
- Instance table page + sort by a KPI returns in well under ~100ms once indexed (warm DB), with **one** HTTP call.
- Filter by status/symbol/text and reorder without downloading the full instance set.
- Opening one instance loads one report; run/cancel/delete still work.

## Implementation map

| Slice | Primary paths |
|-------|----------------|
| Schema + Go write | `src/backtest/store/`, `run_experiment.go`, `runner/instance_report.go` |
| Catalog + HTTP | `src/frontend/lib/experiment-catalog.ts` (removed with the previous Next app), `app/api/v1/experiments/**` (removed) |
| Create + materialize | `POST /experiments`, `createStudy` / `materializeStudy` |
| UI | `experiments-panel.tsx`, `experiment-instance-table-columns.tsx` |
