# cmd

Entrypoints for tools that are not a data job (`src/ingest/cmd`, `src/calculation-technical-indicator/cmd`) or the orchestrator (`src/orchestrator/cmd`).

- `backtest`: container `dagobert-backtest`. With `EXPERIMENT_ID` it runs an experiment phase (`src/backtest/README.md`); otherwise the older single backtest.
- `experiment-api`: standalone Experiment API server (`src/experimentapi`) with empty run hooks. The orchestrator normally serves the API instead.
- `experiment-worker`: AWS Lambda handler (`aws-lambda-go`) for running an experiment step on the serverless path.
- `migrate-candles`: converts a lake's legacy string candle files to the decimal schema; idempotent. `task data:migrate-candles`.
- `seed-demo-year`: resets `DATA_DIR`, backfills a year of BTCUSDT and replays `ma-cross` over it with the simulated venue. `task seed-demo-year`.
