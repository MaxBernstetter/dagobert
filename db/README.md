# db

Postgres schema for the control plane: experiments, instances, period stats, performance, and the connector state (`provider`, `provider_pair`; secrets stay in files). Fills, orders, journals and trade outcomes are Parquet files in each instance workspace, not tables. Managed with Atlas; queries compiled with sqlc. Not used for the Parquet lake or the per-job SQLite files.

## Layout
- `schema/`: the desired state as Atlas HCL (`*.pg.hcl`), grouped `experiment/`, `activity/` and `connector/`; `atlas.hcl` defines the `local` env.
- `migrations/`: generated, timestamped SQL plus `atlas.sum`. Never edit by hand; `atlas.sum` is a checksum.
- `sqlc/`: `queries/*.sql`, `schema.sql` and `sqlc.yaml`; output goes to `src/experimentapi/experimentdb`.

## Workflow
1. Edit the HCL in `schema/`.
2. `task db:diff name=<change>` generates a migration (uses a dev Postgres via Docker). Known limit: the dev database replays `20261024140000_revoke_app_table_api_grants.sql`, which needs the Supabase `anon` role, so the diff fails on a plain Postgres. Then write the migration by hand, check it on a scratch database (`db/sqlc/schema.sql` plus the new file), and run `atlas migrate hash --dir file://migrations`.
3. `task db:migrate` applies migrations to `DATABASE_URL` (starts compose Postgres via `task db:up`). `task db:migrate:supabase` targets `SUPABASE_DB_URL`.
4. If queries or the schema changed for Go: update `sqlc/schema.sql` and `queries/`, then `task db:sqlc`. Commit the generated Go.

`task db:up` only starts the Postgres service; whether `task up` migrates is not automated, so run `task db:migrate` once.

## Before changing
Migrations are append-only once merged. A rename is a new migration (see `20261024120000_rename_study_to_experiment.sql`). Keep `sqlc/schema.sql` in step with the HCL.

Consumers: `src/experimentapi` (see its README) and `src/backtest`.
