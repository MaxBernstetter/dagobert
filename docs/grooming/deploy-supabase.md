# Deploy — Supabase

First technical overview of whether this system can run on Supabase. Not a migration plan. Product facts below are from the current official docs (Edge Functions limits, Cron, Storage, Compute and Disk, Storage pricing, [pricing](https://supabase.com/pricing)). Decisions already made for M1–M6 are not reopened.

## What Supabase can host

One hosted Postgres project, with Auth, Storage, Realtime, and Edge Functions included and optional. A project is one database. Paid plans start at the Pro subscription, from $25 per month, which includes $10 of compute credit (one Micro instance: shared CPU, 1 GB RAM) and 8 GB of database disk. Storage is an object store with an S3-compatible API (PutObject, GetObject, ListObjectsV2, multipart). Cron is `pg_cron`: SQL or an HTTP call, recorded in `cron.job_run_details`, with a documented guidance of at most eight concurrent jobs and ten minutes each. Secrets for Edge Functions are project secrets (100 per project, 48 KiB each).

## What it cannot host

| Need in this system | Supabase |
|---|---|
| Long-running Go job containers | No. Edge Functions are Deno workers: 256 MB, 2 seconds of CPU per request, wall clock 150 seconds (Free) or 400 seconds (paid). They do not run a Docker image, and the wall-clock ceiling is not raisable on the hosted platform. |
| Lambda starting a Fargate task | No container runner and no `RunTask`. |
| EventBridge Scheduler and Step Functions | Cron can fire SQL or HTTP. It has no state machine, no per-state retry that leaves earlier stages done, and no execution history of the kind M6 reads. |
| Parquet lake on S3, emulated by Floci | Storage can hold bytes and speaks a subset of S3. It is not the Floci S3 endpoint the ingest and process containers already use, and it has no versioning. |
| DynamoDB on-demand conditional writes | No DynamoDB. The equivalent is a Postgres insert that does nothing when the key exists. |

## Fit

| Piece | Fit |
|---|---|
| Job runner (M1–M4) | No. The runner is a stateless Go container that exits 0, 1, or 2 plus a job-owned detail code. An Edge Function is a different process, a different language, and a hard time limit. |
| Parquet | Poor. Closed 1-hour klines can be uploaded as objects (paid plans allow a global file limit up to 500 GB). The jobs would still have to change their S3 client target, and local system tests would no longer use Floci. |
| Per-job store | Possible, as Postgres. See below. |
| Secrets | Partial. `BYBIT_API_KEY` and `BYBIT_API_SECRET` can sit in Edge Function secrets or in the database. They cannot be injected into a Fargate task, because there is no task. The Next.js app still must not hold them. |
| Schedule (M5) | No. `pg_cron` is a clock inside Postgres. It does not start four containers in order, retry only the failed stage, or leave a Step Functions history for the pipeline view. |
| Next.js (M6) | Not hosted here. Supabase does not run the app. The three views could read Postgres through the Data API. The pipeline view could not: its record is Step Functions execution history, which this platform does not write. |
| System tests | Stay outside. A case builds the job image, starts the container, and checks the exit code and the records. A Supabase project is not that fixture. |

## Stores

Postgres can replace the four SQLite files and the four DynamoDB tables without merging them and without splitting the KPI row.

One Supabase project is one database. One-store-per-job maps to four schemas in that database, each with its own role. A job connects as its role and can write only its schema. It still reads by primary key and inserts only when the key is absent (`INSERT … ON CONFLICT DO NOTHING`). A conflict is the existing abort: general `2`, detail `1`.

| Store today | Supabase feature |
|---|---|
| `ingest.sqlite` / ingest DynamoDB table | Schema `ingest`. Key: symbol plus open time. Parquet stays the candle body. |
| `process.sqlite` / process table | Schema `process`. One row per symbol and 3-hour bar open. Bar columns plus one `jsonb` value holding the indicator parameter objects and their results. Not a table per indicator. |
| `strategy.sqlite` / strategy table | Schema `strategy`. Actions only. Key: playbook, symbol, candle close. |
| `execution.sqlite` / execution table | Schema `execution`. Key: `orderLinkId`. Item is the fill, the rejection, or the settled no-trade. |

M3 still owns the KPI row's shape and does not own the process schema. M2 writes that row. That split is unchanged. Realtime and Auth do not map to any of these stores.

## Cost shape

A few writes per 3-hour candle is on the order of tens of items a day (three symbols, four jobs, eight closes). DynamoDB on-demand bills per request. At the public on-demand rates (about $1.25 per million write request units and $0.25 per million read request units in us-east-1), that volume is cents per month, plus the tables themselves cost nothing while idle.

Supabase bills a floor. One Pro project on Micro compute is $25 per month before extra disk, egress, or storage. The writes are inside that floor. Storage for a small Parquet set sits inside the 100 GB Pro quota ($0.0213 per GB after that). The database is not the expensive part. The subscription is, and it still does not include the container runner.

Idle DynamoDB is the cheaper store for this cadence. Supabase is cheaper only if the project already pays for that Postgres for something else.

## What stays outside

Even if the four schemas lived in Supabase:

- The four Go images, ECR, and the Lambda that starts the container due.
- EventBridge Scheduler and Step Functions, and Floci locally.
- Parquet on S3 (Floci locally), unless the object client is deliberately pointed at Storage.
- Bybit EU demo (`api-demo.bybit.eu`) and the execution-only secrets.
- Binance public klines (`data-api.binance.vision`). No key.
- The Next.js app. It can read the database. It is not deployed as a Supabase product, and it does not receive exchange secrets.
- System tests. They run the finished container.

## Recommendation

Do not deploy this system on Supabase. The PoC is four stateless Go containers, a Parquet lake, and a Step Functions graph. Supabase's compute is a short Deno worker and a Postgres clock. Using only the database would replace a store that already matches the write-if-absent rule, at a monthly floor this write rate does not need, and would leave the runner and the schedule on AWS anyway.

## Open questions

- If a later read-heavy dashboard outgrows direct DynamoDB reads, is a replica into Postgres worth a store the jobs do not write?
- Does any Supabase feature become useful only after the slice, when M6 needs accounts for operators?
- Local Postgres instead of SQLite is a different question from Supabase hosting, and is not decided here.
