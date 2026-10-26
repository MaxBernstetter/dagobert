# execution

The last job in the live pipeline and the only container with Bybit credentials. It reads the market action, places the paper order on the Bybit EU demo host, and stores the fill in `execution.sqlite`. Layout: `cmd/execution` (entrypoint), `internal/` (job logic, `venue/` clients `bybit`, `paper`, `sim`), `runner/` (container contract: args, env, exit codes 0/1/2).

Run through the orchestrator (`task run`). Test: `go test ./tests/system/execution -count=1` (fake Bybit; needs docker). `venue/sim` is also used by backtests, so changes there affect experiment results. Details: `docs/architecture/pipeline-data-flow.md`.
