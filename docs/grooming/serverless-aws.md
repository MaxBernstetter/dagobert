# Grooming: serverless experiments plane (Floci + AWS + Postgres)

## Goal

Run **experiments** without a live candle clock: Floci locally for AWS services (`:4566`), S3 lake, SSM secrets, batched experiment compute, and a **Postgres control plane** (Compose locally, **Supabase Free** in prod). Browser talks only to an **Experiment API** — never to the DB or lake directly.

| Plane | Local | Prod |
|-------|--------|------|
| Lake / artifacts | Floci S3 | AWS S3 |
| Secrets | Floci SSM (+ `.secrets` fallback) | SSM |
| Control plane (catalog, stats, actions) | Compose **Postgres** | **Supabase Free** (`public`) |
| Experiment compute | Docker / batched workers; SFN+Lambda scaffolding | Batched Lambda Map |
| UI | Next BFF (interim) → Vite SPA | Static SPA + Experiment API |

Detail for Atlas smells/spike: [`atlas-schema.md`](atlas-schema.md). Why not host *jobs* on Supabase: see Out of scope below. API list shape: [`experiment-api-scale.md`](experiment-api-scale.md).

### Live candle pipeline — out of scope

Do not migrate the scheduled candle path (orchestrator clock, candle SFN chain, live pipeline SQLite). Leave dormant or remove; separate track later.

### Frontend target — Vite SPA + Experiment API (not Next forever)

The interim Next BFF (`better-sqlite3`) was removed with step 13: the SPA talks to the Go Experiment API only. End state: static Vite + React; Go Experiment API (sqlc) → Postgres + S3/SSM. No OpenNext.

---

## Storage boundaries (locked)

### Postgres (normalized rows only — **no documents**)

- `experiment` / `experiment_instance` (typed settings; **not** `jsonb` report/grid blobs)
- Experiment **statistics** (`metric_*`, optional period-stat tables)
- Working memory: signals, actions, fills (Activity)
- Secrets stay in **SSM**

### Lake / files (S3 / Floci)

- Candles + TI parquet
- `report.json` optional debug only — **UI must not require it**

### Charts (locked)

| Series | Store | Path |
|--------|--------|------|
| OHLC | Lake parquet | SPA → API → S3 |
| TI overlays | Lake TI parquet | SPA → API → S3 |
| Actions / markers | Postgres | SPA → API → SQL |
| KPI cards / table sort | Postgres `metric_*` | SPA → API → SQL |

### Access

```text
SPA → Experiment API → Postgres | S3/Floci | SSM | orchestrator/SFN
```

Forbidden: SPA → PostgREST / Supabase JS / direct SQL / long-lived S3 keys.  
Floci emulates API Gateway, Lambda, S3, SSM, SFN — **not** Postgres (Compose/Supabase).

---

## Why this order

1. Infra + secrets + lake with little app change (done).
2. Catalog experiment on DDB (done, **superseded** by Postgres).
3. **Postgres control plane** (steps 7–8 done) → **Experiment API (step 9)** ✅
4. Point experiment jobs / fan-out writers at Postgres; charts from lake via API.
5. Vite SPA; harden AWS fan-out.

---

## Step ladder

| # | Difficulty | Step | Status |
|---|------------|------|--------|
| **0** | — | Compose + Floci baseline | ✅ |
| **1** | Easy | Terraform: S3 + SSM + IAM on Floci | ✅ |
| **2** | Easy | Secrets → SSM | ✅ |
| **3** | Easy | `report.json` → S3 (dual-write) | ✅ |
| **4** | Medium | Experiment catalog → DynamoDB | ✅ **superseded**; Terraform tables removed |
| **5** | Medium | Parquet lake → S3 | ✅ |
| **6** | Hard | Experiment fan-out scaffolding (batched / SFN) | ✅ scaffolding; writers still SQLite/DDB |
| **7** | Medium | **Postgres Compose + Atlas migrate Task** | ✅ |
| **8** | Medium | Full normalized HCL (experiments + activity) | ✅ |
| **9** | Hard | sqlc + Go Experiment API (`/v1/experiments*`) | ✅ |
| **10** | Hard | Jobs write Postgres stats/actions; UI drops `report.json` | ✅ |
| **11** | Medium | Chart API → parquet (S3/Floci) | implemented |
| **12** | Medium | ~~Supabase Free~~ superseded: Aurora PostgreSQL Serverless v2 via Terraform, same migrations | Aurora on Floci done; prod migrations need an in-VPC job |
| **13** | Hard | Vite SPA → API only (drop Next BFF) | implemented |
| **14** | Hard | Fan-out workers write Postgres (finish step 6) | ✅ implemented |

**Parked (live pipeline):** old candle Lambda/SFN/SQLite migration.

---

### Steps 0–6 — done (summary)

| Step | Exit / notes |
|------|----------------|
| **0** | `task up` |
| **1** | `task infra:local` → Floci S3/SSM (no DynamoDB tables) |
| **2** | `task infra:seed-ssm`; `SECRETS_BACKEND=auto\|ssm` |
| **3** | `EXPERIMENT_ARTIFACTS=s3` dual-write reports |
| **4** | Terraform DynamoDB tables removed. `EXPERIMENT_CATALOG=sqlite\|dual\|ddb` app dual-write remains; do not invest further |
| **5** | `LAKE_BACKEND` / `LAKE_DUAL_WRITE`; `src/internal/lakeobject` |
| **6** | `EXPERIMENT_RUN_BACKEND=docker\|batched\|sfn`; phases prepare/simulate/finalize; see fan-out knobs below |

#### Step 6 fan-out knobs (retained)

```bash
EXPERIMENT_RUN_BACKEND=docker|batched|sfn
EXPERIMENT_BATCH_SIZE=64
EXPERIMENT_MAP_MAX_CONCURRENCY=8
EXPERIMENT_SFN_ARN=…          # when sfn
```

Golden middle: **50–200 instances / worker**, **MaxConcurrency ~100–400** — not 1:1 Lambda at 20k. After step 10/14, workers update **Postgres** (not DDB) for status/metrics; lake still holds parquet.

---

### Step 7 — Postgres + Atlas ✅ implemented

**What:** Compose `postgres` service; `DATABASE_URL`; Task `db:diff` / `db:migrate` using Atlas OSS.

```bash
# Host (Atlas / psql)
DATABASE_URL=postgres://trading:trading@127.0.0.1:11432/trading?sslmode=disable&search_path=public

task db:up          # start postgres
task db:migrate     # atlas migrate apply --env local
task db:diff name=my_change   # after editing db/schema/**/*.pg.hcl
```

**Code / layout:** `db/atlas.hcl`, `db/schema/**/*.pg.hcl`, `db/migrations/`.  
**Tooling notes:** [`atlas-schema.md`](atlas-schema.md) (OSS ✅; no Pro `docker` block; bare `schema "public" {}`; never transaction pooler).

**Exit:** `task db:migrate` creates `public.experiment` / related tables on Compose Postgres; `atlas` schema holds revisions.

---

### Step 8 — Full normalized schema ✅ implemented

**What:** Complete HCL for experiments, instance metrics, period stats, actions/fills — **no `jsonb` documents**. Indexes on `(experiment_id, metric_*)`.

| Area | Tables |
|------|--------|
| Catalog | `experiment`, `experiment_instance` (typed settings + playbook cfg columns + `metric_*`) |
| Period stats | `experiment_instance_period_stat` |
| Activity | `experiment_instance_action`, `experiment_instance_fill` |

**Exit:** Schema reviewable under `db/schema/`; `task db:migrate` applies cleanly (verified local).

---

### Step 9 — Go Experiment API + sqlc ✅ implemented

**What:** Go Experiment API (`src/experimentapi`, sqlc → Postgres) on orchestrator `:8090` when `DATABASE_URL` is set; optional `go run ./src/cmd/experiment-api` on `:8091`. Next may proxy via `EXPERIMENT_API_URL`.

```bash
task db:migrate
# curl http://127.0.0.1:11090/api/v1/experiments
# Frontend (Compose): EXPERIMENT_API_URL=http://orchestrator:8090
```

**Code:** `src/experimentapi/`, `db/sqlc/`, orchestrator `Mount`. The Next-side proxy was removed with the previous Next app.

**Exit:** Create/list/progress/instances against Postgres (verified via curl).

---

### Step 10 — Job writers → Postgres ✅ implemented

**What:** `RunExperiment` prepare/simulate/finalize use **Postgres** when `DATABASE_URL` is set (`src/experimentapi/pgcatalog`); otherwise SQLite catalog unchanged. Jobs upsert instance `metric_*`, `experiment_instance_period_stat`, and fills; actions deferred. Experiment docker runs join the Compose network so job containers reach `postgres`: `COMPOSE_NETWORK` if set, else the orchestrator's own network (the project name follows the checkout directory), else `dagobert_default`.

**Exit:** Create experiment via Experiment API → run → instance table KPIs/progress from Postgres (no `report.json` for list metrics). `experiment_kpi` bar bodies still SQLite until step 11.

---

### Step 11 — Chart API → lake

**What:** `…/chart-series` reads OHLC+TI from parquet via API (S3/Floci). Actions from Postgres for markers. The handler reads lake parquet and Postgres actions and does not read experiment_kpi.

**Exit:** Experiments chart works without `experiment_kpi` JSON bodies.

---

### Step 12 — Supabase Free — repo side implemented

**What:** Same `db/migrations` on a Supabase Free project. Connect with direct Postgres (`:5432`) or the session pooler (port `5432`, IPv4). Do not use the transaction pooler (`:6543`). `REVOKE` anon, authenticated, and PUBLIC on app tables; no grant to anon.

**Repo:** Implemented. `db/migrations/20261024140000_revoke_app_table_api_grants.sql` revokes those privileges on `experiment`, `experiment_instance`, `experiment_instance_period_stat`, `experiment_instance_action`, and `experiment_instance_fill`. `env "supabase"` in `db/atlas.hcl` reads `SUPABASE_DB_URL`. `task db:migrate:supabase` runs `atlas migrate apply --env supabase` when that variable is set, and exits with an error when it is unset. `env.example` has an empty `SUPABASE_DB_URL=` placeholder.

**Apply (when you have a project):** set `SUPABASE_DB_URL` in `.env` to your direct or session-pooler URI (`sslmode=require`, database `postgres`). Do not commit the URL. From the repo root run `task db:migrate:supabase`. That applies every file in `db/migrations` and records revisions in schema `atlas`. A live apply still needs your Supabase URL; this repo does not call a project and does not ship credentials.

**Exit:** Prod catalog on Supabase; PostgREST cannot read app tables as anon. Live apply is still waiting on your Supabase URL.

---

### Step 13 — Vite SPA ✅ implemented

**What:** Static UI at `src/frontend` (Vite + React). The browser talks only to the Go Experiment API (`VITE_EXPERIMENT_API_URL`). No `better-sqlite3`, no SQLite reads, and no `DATA_DIR` mount in the browser. The previous Next app was removed. The published `WEB_PORT` service is the SPA.

**Exit:** Browser never mounts `DATA_DIR`. The SPA exists at `src/frontend`.

---

### Step 14 — Fan-out → Postgres

**What:** Batched and Step Functions experiment workers write instance status and metrics through the Postgres catalog (`DATABASE_URL`, same contracts as `src/experimentapi/pgcatalog` and `src/backtest/run_experiment.go`). There is no DynamoDB catalog. Step 4's tables and the `sqlite|dual|ddb` dual-write are gone; experiment status and metrics are Postgres only.

**Exit:** 20k-instance experiment on Floci/AWS with peak concurrency in the hundreds; catalog consistent in Postgres.

---

## Acceptance (end state)

- Local: Floci + Compose (Postgres + API + optional Next/Vite); experiments without live candle clock.
- Catalog/stats/actions in Postgres; OHLC/TI charts via API→lake; no anon Data API.
- Experiment fan-out: batched workers.
- UI: static SPA + Experiment API.

## Out of scope

- AWS RDS (use Compose + Supabase Free)
- Hosting jobs or primary lake on Supabase. Edge Functions are Deno workers (256 MB, 2 s CPU per request, 150 s wall clock on Free, 400 s paid) and cannot run a Docker image. `pg_cron` fires SQL or HTTP only: no state machine, no per-stage retry. Storage speaks a subset of S3 and is not the Floci endpoint the jobs use. Supabase is used for Postgres only; its Pro floor ($25/month) also outweighs the write volume of the live pipeline.
- OpenNext / Next as long-term server
- Live candle pipeline migration
- Lowering 20k cap
- Atlas Pro as a hard dependency

## Repo anchors

- `db/` — Atlas schema + migrations (step 7+)  
- `infra/terraform/` — Floci/AWS data plane + experiment SFN scaffolding  
- `docs/grooming/atlas-schema.md` — Atlas feasibility / blockers  
- `docs/grooming/experiment-api-scale.md` — HTTP resource shape  
- `src/frontend/` — Vite SPA (step 13)  
