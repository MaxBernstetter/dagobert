# Protobuf + Buf

Relational control-plane entities and Experiment API / worker contracts are defined in
`proto/tradingcopilot/v1/` (`package tradingcopilot.v1`). Generated code is the
application model; Postgres/Atlas columns are a 1:1 projection of those fields.

## Layout

| Path | Role |
|------|------|
| `proto/buf.yaml` | Module `buf.build/maxbernstetter/dagobert`, STANDARD lint, FILE breaking |
| `proto/buf.gen.yaml` | Go → `src/gen/tradingcopilot/v1`, TS → `src/frontend/src/lib/gen/tradingcopilot/v1` |
| `src/experimentapi/persist` | sqlc rows ↔ `v1` messages only |
| REST `/api/v1/*` | `protojson` (camelCase). `GET .../chart-series` returns binary `ChartSeries` (`application/protobuf`); it is a plot, not a table. |
| Experiment worker / SFN | `ExperimentWorkerBatch` / `ExperimentSFNInput` (protojson or binary protobuf) |

## Commands

```bash
task proto:lint
task proto:gen
task proto:breaking   # against main (or BSR after first push)
task proto:push       # requires login + module (see below)
```

### BSR (first time)

```bash
buf registry login
buf registry module create buf.build/maxbernstetter/dagobert --visibility private
cd proto && buf push --label v0.1.0
```

## Versioning

1. New fields get new numbers only; never renumber or reuse.
2. Removals: `deprecated = true` and `reserved` the number; drop the Atlas column in the same change.
3. Incompatible shape → `tradingcopilot.v2`.
4. Tag BSR versions (`v0.1.0`, …) when pushing intentional releases.

## Adding a relational field

1. Add the field to the matching `tradingcopilot.v1` message.
2. Add the Atlas HCL column (`db/schema`).
3. `task db:diff` / `task db:migrate` if needed; update sqlc queries.
4. Map in `src/experimentapi/persist`; regenerate with `task proto:gen`.
