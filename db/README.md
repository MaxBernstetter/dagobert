# db

Postgres schema for the control plane: experiments, instances (status and aggregated metrics), experiment performance, the ingest ledger (`ingest_stored`, `ingest_unavailable`) and the connector state (`provider`, `provider_pair`; secrets stay in files). Fills, orders, journals, trade outcomes, period stats and the instance report are Parquet row groups in shard files (`experiments/<id>/shards/<shard>/`), not tables; `experiment_instance.shard_key` is the shard directory of a completed instance (empty until then) and `workspace_rel_path` its workspace directory (paper wallet; key of the S3 report copy). Managed with Atlas; queries compiled with sqlc. Not used for the Parquet lake or the per-job SQLite files.

## Layout
- `schema/`: the desired state as Atlas HCL (`*.pg.hcl`), grouped `experiment/`, `activity/`, `connector/` and `ingest/`; `atlas.hcl` defines the `local` env.
- `migrations/`: generated, timestamped SQL plus `atlas.sum`. Never edit by hand; `atlas.sum` is a checksum.
- `sqlc/`: `queries/*.sql`, `schema.sql` and `sqlc.yaml`; output goes to `src/experimentapi/experimentdb`.

## Workflow
1. Edit the HCL in `schema/`.
2. `task db:diff name=<change>` generates a migration (uses a dev Postgres via Docker). Known limit: the dev database replays `20261024140000_revoke_app_table_api_grants.sql`, which needs the `anon` role (created by `task db:up` on the Aurora database, absent on a plain Postgres), so the diff fails on a plain Postgres. Then write the migration by hand, check it on a scratch database (`db/sqlc/schema.sql` plus the new file), and run `atlas migrate hash --dir file://migrations`.
3. `task db:migrate` applies migrations to `DATABASE_URL` (default: the Aurora cluster on Floci; `task db:up` applies Terraform first). Prod Aurora is private to its VPC, so prod migrations cannot run from a dev machine or a CI runner outside the VPC; that is open until a migration job runs inside the VPC.
4. If queries or the schema changed for Go: update `sqlc/schema.sql` and `queries/`, then `task db:sqlc`. Commit the generated Go.

`task up` and `task dev` run `task db:migrate`.

## Before changing
Migrations are append-only once merged. A rename is a new migration (see `20261024120000_rename_study_to_experiment.sql`). Keep `sqlc/schema.sql` in step with the HCL.

Consumers: `src/experimentapi` (see its README) and `src/backtest`.
