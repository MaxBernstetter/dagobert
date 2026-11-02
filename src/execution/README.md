# execution

The simulated venue layer shared by experiments and the demo replay. There is no container and no real exchange: orders are filled in memory against the paper wallet with the cost model from `internal/venue/sim` and stored in `execution.sqlite` (per-instance workspace). Layout: `internal/job` (order and journal shapes), `internal/store` (SQLite tables), `runner/` (`SimulateBar`, `SimBatch`: one bar's decision to fill/rejection rows).

`venue/sim` drives experiment results, so changes there change every report. Covered by `tests/system/experiment` and the unit tests in `src/internal/venue/sim`.
