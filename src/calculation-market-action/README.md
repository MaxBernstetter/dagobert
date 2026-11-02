# calculation-market-action

Position sizing for simulated runs. `internal/allocate` turns a signal plus cash, holdings and mark price into a sized decision (`ALLOC_*` environment settings override the defaults); `runner/` exposes `PlanAction` to the experiment engine. There is no container: the engine calls it in-process for every bar.

Covered by the unit tests in `internal/allocate` and the sizing tests in `tests/system/experiment`. Details: `src/calculation-signal/playbooks/position-sizing/DESCRIPTION.md`.
