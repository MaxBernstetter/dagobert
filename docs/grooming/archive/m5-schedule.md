# M5 — Scheduled run

First technical overview of the live 3-hour schedule. Not a detailed design. Four Go services stay four containers: ingest, process, strategy, execution. Nothing in this milestone merges them into one process, and nothing here is Apache Airflow.

One closed 3-hour UTC candle is one run. Boundaries are 00:00, 03:00, 06:00, 09:00, 12:00, 15:00, 18:00, and 21:00 UTC. The run input is that close instant. Order inside the run is always ingest, then process, then strategy, then execution. A later backfill may fan out with a Step Functions Map state. The live path is a single sequential execution. Map is not part of this pass.

## What starts what

**AWS**

EventBridge Scheduler is the clock. On each candle close it starts one Step Functions execution and passes the close instant. It does not start containers itself.

The state machine has four states in that fixed order. Each state invokes a Lambda whose only job is to start the Fargate task (`RunTask`) for the service that is due, then report when that task has exited. The Lambda does not ingest, process, decide, or trade. The same Lambda image is used for every state; the state passes which service to start. The next state starts only after the current task has exited successfully.

Images live in ECR. Ingest reads and writes Parquet on S3. Process, strategy, and execution use DynamoDB for the rows they own. Secrets stay in Secrets Manager. The containers are the same images that run locally.

**Local**

Floci on `localhost:4566` is the Step Functions endpoint. The orchestrator container registers the state machine and polls its activity. It starts the four job images with the host Docker socket and waits until each exits. Exit `2` is success. The clock lives in that orchestrator, at the eight UTC boundaries. Persistence for this stack is the `./data` volume (Parquet and the four SQLite files). Floci DynamoDB and S3 are not wired.

The emulated scheduler in the original sketch is that orchestrator clock. Each state runs the matching container. Local object and table storage stay files on `./data`.

## Retry without redoing a finished stage

Retry is configured on each state, not on the whole execution. If process fails, Step Functions runs process again. Ingest already succeeded, so it is not started again. The same rule applies to strategy and execution.

A state retries only its own task. A successful write from an earlier state is left in place.

Redrive of a failed execution is not available on the local emulator, so the design does not depend on it. If an execution ends in failure, recovery is a new execution for the same close instant. That second execution is safe only because each service treats an already-written candle as a no-op (next section). On AWS the same new-execution path is the one we rely on. Per-state retry still covers transient failures before the execution is marked failed.

## Idempotent candle close

The identity of a run is the candle close instant. Every service keys its write to that instant. A repeated close, whether from a state retry that already wrote part of its work or from a brand-new execution, must not append a second copy.

- **Ingest** writes the closed Binance 1-hour klines that belong to that window to a stable object key. Writing again replaces that object. It does not append a second file for the same hours.
- **Process** writes one KPI row per symbol for that close. The item key is symbol plus close instant. Writing again replaces that item.
- **Strategy** emits one action for that close, including an explicit no-trade. The action key is the close instant. A second run returns the stored action and does not emit another.
- **Execution** submits at most one order for that action. If the fill or the rejection is already stored, it does not call Bybit again.

A state retry after a successful write therefore hits the same keys and stops. A later execution of the same close walks all four states and each one no-ops.

## Open questions

- How a state waits for Fargate: Step Functions task sync on `RunTask`, or the Lambda polling until the task exits. The Lambda still only starts the container.
- How long after the UTC boundary the schedule fires, so the last 1-hour kline is actually closed at Binance before ingest runs.
- Whether a run still in progress blocks the next candle's execution, or the two may overlap.
- Execution's crash window: the order was accepted by Bybit and the process died before the fill was stored. A retry must not send a second order. Where that idempotency key lives is still open.
