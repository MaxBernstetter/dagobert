# Status

As of 2026-11-01 (branch `feat/164-experiment-parquet-storage`, PR #202, draft; main at e7b94a3). Update this page whenever an epic closes or the plan changes.

## What works today

- **Live pipeline.** Five Go job containers per 3-hour UTC candle: ingest, technical indicators, signal, market action, execution. The orchestrator runs them in order and places a paper order on the Bybit EU demo host.
- **Experiments.** Create an experiment (manual list or grid of instances) through the Experiment API, run it, read results in the frontend. The Go engine is the default. The C++ engine is opt-in with `EXPERIMENT_ENGINE=cpp` and covers `ma-cross` and `signal-consensus`; it is equivalence-tested against Go (`TestSmaCrossGoVsCppEquivalence`, `TestConsensusGoVsCppEquivalence`).
- **Decimal candles.** Parquet candles use fixed-scale decimals; the reader also accepts the old string schema (`task data:migrate-candles`).
- **Position sizing.** Unit risk, heat and max units, in both engines ([position-sizing](../src/calculation-signal/playbooks/position-sizing/DESCRIPTION.md)).

## Deliberately not done

- The live pipeline does not use C++. `CPP_*_BIN` is read only by the experiment engine (`src/backtest/cpp_engine.go`).
- The Go jobs are not deleted. That waits until the live path runs on C++ (stated in closed epic #54).
- The trader overview UI was dropped (milestone M6 closed as not planned).
- Production deployment is not evidenced as running. The repo has Terraform for AWS and a Supabase project (`infra/terraform`, `task infra:prod`, `task db:migrate:supabase`), but `docs/grooming/serverless-aws.md` says a live Supabase apply still waits on a project URL, and nothing in the repo records an applied prod stack. Treat prod as prepared, not confirmed.

## Measured performance

The figures in [05-cpp-benchmark](development/05-cpp-benchmark.md) were measured before #164 (SQLite catalog, Postgres fill COPY) and are kept as history. Since #164 nothing is measured: the Parquet instance files have no numbers yet (TODO in that document).

## Next steps, in priority order

1. Measure the instance Parquet files: write cost per instance, file size, `/activity` and `/chart-series` latency, on a 3000-instance run ([#164](https://github.com/MaxBernstetter/dagobert/issues/164), PR #202). Record the numbers in [05-cpp-benchmark](development/05-cpp-benchmark.md).
2. Make C++ the default experiment engine, then move the live path to C++ and delete the Go jobs. No issue yet; needs more validation first (see [architecture/backtesting.md](architecture/backtesting.md)).
3. Finish the serverless-aws ladder ([serverless-aws.md](grooming/serverless-aws.md)): steps 11 (chart API from the lake), 12 (Supabase, needs a real project URL) and 14 (20k-instance run on Floci or AWS) are marked implemented but their exit checks are not recorded as done.

Open research issues [#1](https://github.com/MaxBernstetter/dagobert/issues/1) and [#2](https://github.com/MaxBernstetter/dagobert/issues/2) are background reading, not planned work.

## Known loose ends

Things that were not verified when this page was written (2026-10-26):

- The mermaid diagrams (README, `architecture/experiment-data-flow.md`, `architecture/pipeline-data-flow.md`) were checked structurally only. Open them on GitHub once and fix any that do not render.
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
4. Quick checks: `go test ./src/internal/... ./src/shared/...` (seconds). `task test` runs all job system tests (about 20 minutes). C++: `task cpp:deps PRESET=debug` once, then `task cpp:test`.
5. Logs: `docker compose logs -f orchestrator` (job output and `DETAIL=<n>`). Data: `./data` (Parquet lake, SQLite files, `experiments/<id>/instances/<id>/` workspaces); Postgres lives in the compose volume.
