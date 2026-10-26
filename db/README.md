# db

Postgres schema for the experiment control plane (experiments, instances, fills, period stats, performance). Managed with Atlas; queries compiled with sqlc. Not used for the Parquet lake or the per-job SQLite files.

## Layout
- `schema/`: the desired state as Atlas HCL (`*.pg.hcl`), grouped `experiment/` and `activity/`; `atlas.hcl` defines the `local` env.
- `migrations/`: generated, timestamped SQL plus `atlas.sum`. Never edit by hand; `atlas.sum` is a checksum.
- `sqlc/`: `queries/*.sql`, `schema.sql` and `sqlc.yaml`; output goes to `src/experimentapi/experimentdb`.

## Workflow
1. Edit the HCL in `schema/`.
2. `task db:diff name=<change>` generates a migration (uses a dev Postgres via Docker).
3. `task db:migrate` applies migrations to `DATABASE_URL` (starts compose Postgres via `task db:up`). `task db:migrate:supabase` targets `SUPABASE_DB_URL`.
4. If queries or the schema changed for Go: update `sqlc/schema.sql` and `queries/`, then `task db:sqlc`. Commit the generated Go.

`task db:up` only starts the Postgres service; whether `task up` migrates is not automated, so run `task db:migrate` once.

## Before changing
Migrations are append-only once merged. A rename is a new migration (see `20261024120000_rename_study_to_experiment.sql`). Keep `sqlc/schema.sql` in step with the HCL.

Consumers: `src/experimentapi` (see its README) and `src/backtest`.
