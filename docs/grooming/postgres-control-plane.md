# Postgres control plane — decisions (folded into serverless plan)

**Canonical ladder:** [`serverless-aws.md`](serverless-aws.md) (steps **7–8** = Compose Postgres + full Atlas HCL ✅; **step 9** = Go Experiment API + sqlc).

This file keeps the locked decisions in one place; do not maintain a second step list here.

| Topic | Decision |
|-------|----------|
| Local DB | Compose Postgres |
| Prod DB | Supabase Free (`public` only) |
| Schema | Atlas multi-file HCL `db/schema/**/*.pg.hcl` + versioned SQL migrations |
| Queries | sqlc (Go Experiment API) |
| UI access | SPA → Experiment API only (no PostgREST / direct SQL) |
| In Postgres | Catalog, stats (`metric_*`), actions/fills — **no document columns** |
| Charts | OHLC + TI from **lake via API**; actions/stats from Postgres |
| DDB catalog | Superseded. Terraform no longer creates DynamoDB tables. App dual-write (`EXPERIMENT_CATALOG`) remains until a follow-up. |

Atlas tooling / spike / smells: [`atlas-schema.md`](atlas-schema.md).  
Do not host jobs on Supabase: [`deploy-supabase.md`](deploy-supabase.md).
