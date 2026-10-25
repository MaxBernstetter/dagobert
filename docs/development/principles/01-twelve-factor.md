# Twelve-factor app

Every Dagobert component follows the [Twelve-Factor App](https://12factor.net/) methodology. The table states what each factor means **in this project**.

| # | Factor | Rule in Dagobert |
|---|---|---|
| I | Codebase | One repository. Each job, the orchestrator and the frontend are separately built images from `docker/` and `src/`. |
| II | Dependencies | Declared and locked: `go.mod` / `go.sum`, `src/frontend/package.json` + lockfile. Nothing relies on packages preinstalled on the host. Toolchain comes from `devbox.json`. |
| III | Config | Everything that differs between environments comes from environment variables (`env.example`, `.env`). No per-environment config files in the repo. Secrets (`.secrets`) reach only the container that needs them (execution). |
| IV | Backing services | Binance, Bybit and Floci/S3 are attached resources addressed by URL or credentials from config (`BINANCE_BASE_URL`, `AWS_ENDPOINT_URL`). Swapping one needs no code change. |
| V | Build, release, run | Images are built from the commit (`task build`); release = image + config; run = `task up` or a one-shot `docker run`. Running containers are never edited. |
| VI | Processes | Jobs are stateless runners. State lives in the `./data` volume (Parquet and SQLite), not in process memory between runs. |
| VII | Port binding | The frontend and orchestrator export HTTP on a configured port (`WEB_PORT`, `ORCHESTRATOR_PORT`). |
| VIII | Concurrency | Scale by running more processes, not bigger ones. Work is split into chunks such as date ranges or instances (`EXPERIMENT_SIM_WORKERS`). |
| IX | Disposability | Fast start, clean exit with a documented exit code (`0`, `1`, `2`). Idempotent work, so a killed job can simply rerun. |
| X | Dev/prod parity | The same Compose stack and job images locally and live. Local runs use Floci, not a different code path. |
| XI | Logs | Each job logs `DETAIL=<n>` for diagnosis; the orchestrator keeps container logs. Do not write log files inside a job. |
| XII | Admin processes | One-off tasks (`seed-demo-year`, backfill, `probe-bybit`) run as versioned `task` targets or `go run` commands against the same code, never by hand in a shell. |

## Checks when planning a piece

- Which configuration does it need, and is it all from environment variables?
- Does it keep any state outside `./data`? If yes, redesign.
- Can it be killed at any moment and rerun safely?
- Does it behave the same locally and live?
