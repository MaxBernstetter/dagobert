# cmd

Entrypoints for tools that are not one of the five pipeline jobs (those live in `src/<job>/cmd`, the orchestrator in `src/orchestrator/cmd`).

- `backtest`: container `dagobert-backtest`. With `EXPERIMENT_ID` it runs an experiment phase (`src/backtest/README.md`); otherwise the older single backtest.
- `experiment-api`: standalone Experiment API server (`src/experimentapi`) with empty run hooks. The orchestrator normally serves the API instead.
- `experiment-worker`: AWS Lambda handler (`aws-lambda-go`) for running an experiment step on the serverless path.
- `migrate-candles`: converts a lake's legacy string candle files to the decimal schema; idempotent. `task data:migrate-candles`.
- `seed-demo-year`: resets `DATA_DIR` and backfills a year of BTCUSDT with `VENUE_SIMULATE=1`. `task seed-demo-year`.
