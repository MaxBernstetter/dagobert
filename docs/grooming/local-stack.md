# Local stack

`task up` should leave three processes running. It should not execute a candle by itself.

## What stays up

| Process | Role |
|---|---|
| Frontend | The Next.js app. Reads `./data`. Does not start jobs. |
| Persistence | The `./data` volume: Parquet and `ingest.sqlite`, `process.sqlite`, `strategy.sqlite`, `execution.sqlite`. Jobs and the app mount the same directory. |
| Orchestrator | Floci on port 4566, and a starter process that owns the Docker socket. |

The four job images are runners. They appear when a candle close is due and exit when that stage is done.

Floci DynamoDB and S3 are not part of this change. The jobs already persist on `./data`. Pointing them at Floci is a later change.

## What the orchestrator does

1. On startup the starter registers one state machine: ingest, then process, then strategy, then execution. The input is the candle close in milliseconds.
2. Each state tells the starter which image to run and which arguments to pass. The starter runs that image, waits until it exits, and returns the code. Exit `0` and exit `2` are success. Exit `1` fails the state. `DETAIL` stays on the container log. Bybit credentials go only to the execution container.
3. Retry is on the state, three attempts, and only after exit `1`. A later state does not start until the current one has exited `0` or `2`.
4. A clock starts one execution at each UTC boundary: 00:00, 03:00, 06:00, 09:00, 12:00, 15:00, 18:00, 21:00. The input is that boundary. If Floci cannot express the schedule, the starter keeps that clock itself. One clock, not both.
5. A second execution of the same close is a new execution. Each job no-ops (`DETAIL=1`).

Floci's Lambda cannot see the host Docker socket. The starter is the process that runs the images. It can live in the compose network with the socket mounted.

## Commands

| Command | Effect |
|---|---|
| `task up` | `docker compose up`. Floci, the starter, and the web app stay up. |
| `task run` | One execution for `CANDLE_CLOSE_MS`. Default `1700006400000`. |
| `task down` | Stops the stack. Does not delete `./data`. |

`compose.yaml` no longer lists the four jobs, and it no longer sets `COMPOSE_OK_ON_ABORT`. The pipeline page keeps reading `pipeline.json` until a later change reads Step Functions history.

## Acceptance

- `docker compose ps` shows Floci, the starter, and the web app. It does not show the four jobs.
- `task run` for a new close runs the four images in order. The app shows the action and, when an order is sent, the fill and the balances.
- `task run` for a close that is already stored prints `DETAIL=1` for each job, sends no second order, and the execution succeeds.
- A job that exits `1` fails that state. The retry does not start the earlier jobs again.

## Out of scope

Switching SQLite and local Parquet to Floci DynamoDB and S3. AWS EventBridge, Lambda, and Fargate. Backfill. The pipeline page reading Step Functions history.
