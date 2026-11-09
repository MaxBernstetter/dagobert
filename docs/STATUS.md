# Status

As of 2026-11-01 (branch `feat/164-experiment-parquet-storage`, PR #202, draft; main at e7b94a3). Update this page whenever an epic closes or the plan changes.

## What works today

- **Scope.** Dagobert is a backtesting framework. The live and paper-trading pipeline (candle-close state machine, Bybit client, execution job, `.secrets`) was removed.
- **Experiments.** Create an experiment (manual list or grid of instances) through the Experiment API, run it, read results in the frontend. Experiments run on the C++ engine only (`ma-cross` and `signal-consensus`); each experiment sets its C++ process count (auto = one per CPU) in the create form; the Go engine and `EXPERIMENT_ENGINE` were removed (#164). The replayed output is checked by `TestSmaCrossCppReplayConsistent` and `TestConsensusCppReplayConsistent`.
- **Decimal candles.** Parquet candles use fixed-scale decimals; the reader also accepts the old string schema (`task data:migrate-candles`).
- **Position sizing.** Unit risk, heat and max units, ([position-sizing](../src/calculation-signal/playbooks/position-sizing/DESCRIPTION.md)).

## Deliberately not done

- `CPP_*_BIN` is read only by the experiment engine (`src/backtest/cpp_engine.go`).
- Leftover naming: simulated rows still carry `venue: bybit` and the `VENUE_SIMULATE` flag; instrument USDC venue mappings in `src/internal/instruments` are unchanged.
- The trader overview UI was dropped (milestone M6 closed as not planned).
- Production deployment is not evidenced as running. The repo has Terraform for AWS (`infra/terraform`: VPC, Aurora PostgreSQL Serverless v2, S3, SSM, IAM; `task infra:prod` runs in CI only). Nothing records an applied prod stack, and prod migrations need a job inside the VPC that does not exist yet. Treat prod as prepared, not confirmed.

## Measured performance

3000 ma-cross instances on the C++ engine, 22 CPUs, throwaway Postgres 16 ([05-cpp-benchmark](development/05-cpp-benchmark.md)): the run takes 14.9 s (zigzag lake, median of three) and 16.9 s (random-walk lake, median of three); all instances complete and the experiment directory is 232 MiB and 484 MiB. The older figures in that document are Go-era history.

## Next steps, in priority order

1. Measure the browser load of the instance detail page (`/activity`, `/chart-series`) on a 3000-instance experiment ([#164](https://github.com/MaxBernstetter/dagobert/issues/164), PR #202). The run and file numbers are in [05-cpp-benchmark](development/05-cpp-benchmark.md).
3. Finish the serverless-aws ladder ([serverless-aws.md](grooming/serverless-aws.md)): steps 11 (chart API from the lake) and 14 (20k-instance run on Floci or AWS) are marked implemented but their exit checks are not recorded as done. Step 12 (Supabase) is superseded by Aurora: the relational database is the first piece migrated to the serverless stack (VPC, API Gateway, C++ Lambda and ECS frontend follow).

Open research issues [#1](https://github.com/MaxBernstetter/dagobert/issues/1) and [#2](https://github.com/MaxBernstetter/dagobert/issues/2) are background reading, not planned work.

## Known loose ends

Things that were not verified when this page was written (2026-10-26):

- The mermaid diagrams (README, `architecture/experiment-data-flow.md`) were checked structurally only. Open them on GitHub once and fix any that do not render.
- `task seed-demo-year` and running an experiment from the UI were not run end to end after the C++ engine and docs work. Do this first when you resume.
- The wiki pages were not opened in a browser after the Next.js removal and the move of wiki prose into markdown (the build passes).
- `terraform fmt` / `validate` were not run on `infra/terraform` after the heavy-path stub removal (no terraform in the dev shell).
- Experiments completed before #164 have no trade outcome files, so their trades are gone from `/kpis` and `/performance` and their activity and charts are empty. A new run rebuilds them; there is no backfill. See [experiment-data-flow](architecture/experiment-data-flow.md#known-limits).
- `src/internal/experimentddb` has no caller since the SQLite experiment catalog was removed (#164). It is kept, not deleted.
- Local-only: `src/frontend/node_modules` may be owned by root if it was created in a container; `npm ci` then fails. Delete it or `chown` it.

## How to pick up work

Follow [AGENTS.md](../AGENTS.md) and [development/01-task-management.md](development/01-task-management.md).

- Pick an issue on [project board #5](https://github.com/users/MaxBernstetter/projects/5). One issue, one pull request, one worktree under `worktree/`.
- Set the issue to `In progress` before planning or coding, `In review` when the pull request is open.
- Keep a pull request under about 600 changed lines.

## How to resume after a break

1. `git pull` on `main`; `devbox shell` for Go and Task.
2. `task build` (job and compose images; needed after pulls that touch `src/`).
3. `task up`, then open the frontend on http://localhost:11001. `task up` does not run migrations: run `task db:migrate` once after a pull that adds files to `db/migrations/`.
4. Quick checks: `go test ./src/internal/... ./src/shared/...` (seconds). `task test` runs all system tests (about 20 minutes). C++: `task cpp:deps PRESET=debug` once, then `task cpp:test`.
5. Logs: `docker compose logs -f orchestrator` (job output and `DETAIL=<n>`). Data: `./data` (Parquet lake, SQLite files, `experiments/<id>/instances/<id>/` workspaces); Postgres lives in the compose volume.
