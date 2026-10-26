# calculation-market-action

Fourth job: sizes the signal against live Bybit balances and writes the market action for `execution`. Layout: `cmd/calculation-market-action`, `internal/` (`allocate` sizing), `runner/` (container contract).

The sizing logic is also exercised by backtests through `ALLOC_*` environment settings, so keep behaviour in sync with the simulated path. No dedicated system test directory exists; it is covered through the pipeline run (`task run`) and sizing tests in `tests/system/experiment`. Details: `src/calculation-signal/playbooks/position-sizing/DESCRIPTION.md`.
