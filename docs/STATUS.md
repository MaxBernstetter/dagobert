# Status

As of 2026-10-26 (main at 448c584). Update this page whenever an epic closes or the plan changes.

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

3000 instances, C++ engine ([05-cpp-benchmark](development/05-cpp-benchmark.md)): about 4-6 s with the SQLite catalog, about 100 s with the Postgres catalog. The Postgres cost is write volume (about 5 M fills per run). That is issue #164.

## Next steps, in priority order

1. Decide how much fill data to persist per instance: [#164](https://github.com/MaxBernstetter/dagobert/issues/164). It sets the Postgres cost of every large experiment.
2. Make C++ the default experiment engine, then move the live path to C++ and delete the Go jobs. No issue yet; needs more validation first (see [architecture/backtesting.md](architecture/backtesting.md)).
3. Finish the serverless-aws ladder ([serverless-aws.md](grooming/serverless-aws.md)): steps 11 (chart API from the lake), 12 (Supabase, needs a real project URL) and 14 (20k-instance run on Floci or AWS) are marked implemented but their exit checks are not recorded as done.

Open research issues [#1](https://github.com/MaxBernstetter/dagobert/issues/1) and [#2](https://github.com/MaxBernstetter/dagobert/issues/2) are background reading, not planned work.

## How to pick up work

Follow [AGENTS.md](../AGENTS.md) and [development/01-task-management.md](development/01-task-management.md).

- Pick an issue on [project board #5](https://github.com/users/MaxBernstetter/projects/5). One issue, one pull request, one worktree under `worktree/`.
- Set the issue to `In progress` before planning or coding, `In review` when the pull request is open.
- Keep a pull request under about 600 changed lines.

## How to resume after a break

1. `git pull` on `main`; `devbox shell` for Go and Task.
2. `task build` (job and compose images; needed after pulls that touch `src/`).
3. `task up`, then open the frontend on http://localhost:3001. `task up` does not run migrations: run `task db:migrate` once after a pull that adds files to `db/migrations/`.
4. Quick checks: `go test ./src/internal/... ./src/shared/...` (seconds). `task test` runs all job system tests (about 20 minutes). C++: `task cpp:deps PRESET=debug` once, then `task cpp:test`.
5. Logs: `docker compose logs -f orchestrator` (job output and `DETAIL=<n>`). Data: `./data` (Parquet lake, SQLite files, `experiments/<id>/instances/<id>/` workspaces); Postgres lives in the compose volume.
