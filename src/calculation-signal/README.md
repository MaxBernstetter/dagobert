# calculation-signal

Third job: runs a playbook over the indicators and stores the signal in `calculation_signal.sqlite`. Also the strategy engine used by experiments.

## Layout
- `cmd/calculation-signal`: entrypoint. `runner/`: container contract plus shared pipeline code.
- `playbooks/<name>/`: `ma-cross`, `signal-consensus` (each `strategy.go`, `DESCRIPTION.md`, tests), and `position-sizing`, `strategy-kpi` (shared pieces).
- `internal/framework`: strategy, position and KPI model. `internal/playbooks/registry.go` maps names to implementations (`Resolve`). `internal/sizing`: unit sizing.
- `schema/playbook.schema.json`: validates playbook config.
- `runner/fast_pipeline.go` is the in-memory experiment loop (Go engine); `runner/kpi_worker.go` (`ReplayKPI`, `RunKPIWorkers`) replays C++ engine actions into reports.

## Test
Playbook and framework unit tests: `go test ./src/calculation-signal/...`. System test: `go test ./tests/system/calculation-signal -count=1`.

## Adding a playbook
Add the package, register it in `registry.go`, update the schema and `DESCRIPTION.md` (the frontend wiki reads it).
