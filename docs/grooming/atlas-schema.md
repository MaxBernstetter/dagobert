# Grooming: Atlas schema-as-code (Postgres)

## Goal

Manage the **relational control plane** (experiments, stats, actions / working memory) as code with [Atlas](https://atlasgo.io): multi-file HCL desired state, versioned SQL migrations, apply to **local Compose Postgres** and **Supabase Free**. Lake (candles / TI parquet) stays on S3/Floci — out of Atlas.

Companion decisions (this track):

- UI → Experiment API only (no browser → DB / PostgREST).
- No document columns (`json`/`jsonb` report bodies); chart OHLC/TI via API → S3.
- Typed queries later via **sqlc** (Go) against the migrated schema.

Not a migration implementation plan for existing SQLite/DDB data — schema tooling only.

## Locked choices

Canonical ladder: [`serverless-aws.md`](serverless-aws.md) (steps **7–8** done; **9** = sqlc Experiment API next).

| Item | Choice |
|------|--------|
| Desired state | Atlas HCL, **multiple files** under `db/schema/**/*.pg.hcl` |
| Migrations | Versioned SQL in `db/migrations/` + `atlas.sum` |
| Local DB | Compose `postgres` service |
| Prod DB | Supabase Free (`public` only) |
| Apply | `atlas migrate apply` via Task / CI — **not** on API process start |
| Revisions table | Schema `atlas` (`revisions_schema = "atlas"`) — keep out of `public` |
| Scope | `search_path=public` / `schemas = ["public"]` — never manage Supabase platform schemas |

## Why Atlas (fit)

- Schema-as-code independent of API language (Go or TS).
- `migrate diff` from desired HCL → reviewable SQL.
- Multi-file via `data "hcl_schema"` + `fileset("db/schema/**/*.pg.hcl")`.
- Official [Supabase guide](https://atlasgo.io/guides/postgres/supabase).
- Works beside sqlc (SQL queries, not an ORM schema).

## Suggested layout

```text
db/
  atlas.hcl                 # envs: local, supabase
  schema/
    _schema.pg.hcl          # schema "public" {}
    experiment/
      experiment.pg.hcl
      experiment_instance.pg.hcl
    activity/
      experiment_action.pg.hcl
      …                     # fills / signals as needed
  migrations/               # generated + reviewed
  baseline/                 # optional stubs for supabase-shaped.devtools only
```

```hcl
# db/atlas.hcl (sketch)
data "hcl_schema" "app" {
  paths = fileset("schema/**/*.pg.hcl")
}

env "local" {
  url = getenv("DATABASE_URL")
  # OSS-friendly: stock Postgres. Prefer this for local Compose.
  dev = "docker://postgres/16/dev"
  src = data.hcl_schema.app.url
  migration {
    dir              = "file://migrations"
    revisions_schema = "atlas"
  }
}

env "supabase" {
  url = getenv("SUPABASE_URL")  # direct :5432 + search_path=public&sslmode=require
  # See blockers: Pro docker block vs OSS stock image
  dev = "docker://postgres/16/dev"
  src = data.hcl_schema.app.url
  schemas = ["public"]
  migration {
    dir              = "file://migrations"
    revisions_schema = "atlas"
  }
}
```

## Smells

### 1. Dual migration authorities (Supabase CLI vs Atlas)

**Smell:** Dashboard SQL editor, `supabase migration`, and Atlas `db/migrations` can diverge.

**Mitigation:** Atlas is the **only** schema writer for app tables. Disable habit of dashboard DDL. Do not also run Supabase CLI migrations for the same objects. Optional: ignore `supabase_migrations` entirely for app schema.

### 2. Managing too much of the Supabase database

**Smell:** `atlas schema inspect` without `schemas = ["public"]` / `search_path` pulls `auth`, `storage`, … then diffs try to “fix” platform objects.

**Mitigation:** Always scope to `public`. Never put platform schemas in desired HCL.

### 3. RLS / Data API as accidental public surface

**Smell:** Supabase exposes `public` via PostgREST. Tables without RLS (or with open policies) are reachable if anon key leaks — even if the SPA “only uses the API.”

**Mitigation (pick one and lock):**

- **A (preferred for this product):** No Data API use — revoke `anon`/`authenticated` privileges on app tables; only `service_role` / API role connects. Document that SPA never embeds the anon key for DB.
- **B:** Enable RLS deny-all for anon on every app table; API uses a privileged DB role.

Atlas can express grants/RLS in HCL; **grant management is Atlas Pro** in the Supabase guide — OSS may need raw SQL migrations for `REVOKE`/`GRANT`.

### 4. HCL vs SQL desired state drift of taste

**Smell:** Team edits generated migration SQL by hand and forgets to update HCL (or the reverse).

**Mitigation:** Workflow is **always** edit HCL → `atlas migrate diff` → review SQL. Ban hand-editing applied migrations; use a new diff for fixes. Protect `atlas.sum`.

### 5. “Schema as code” that still embeds documents

**Smell:** Adding `jsonb` “for convenience” (grid, report, KPI body) reopens the document ban.

**Mitigation:** Lint in review; optional CI grep on `db/schema` for `jsonb` / `json`.

### 6. Dev database image mismatch

**Smell:** Diff planned on stock `postgres:16` but prod is Supabase (extensions, roles, non-superuser). Policies calling `auth.uid()` or defaults using `extensions.uuid_generate_v4()` fail to plan or plan wrong.

**Mitigation for this app (API-only, no Supabase Auth policies initially):** keep **stock Postgres** for `dev` and avoid Auth-dependent DDL in desired state. If later you add RLS with `auth.uid()`, you need a Supabase-shaped baseline or `supabase/postgres` image (see blockers).

### 7. Auto-migrate on API boot

**Smell:** Lambda/Compose API runs migrations on startup → lock races, multi-replica double-apply, surprise DDL in prod.

**Mitigation:** Explicit `task db:migrate` / CI job only.

### 8. sqlc and Atlas out of sync

**Smell:** Migrations applied but `sqlc generate` not run → runtime/query mismatch.

**Mitigation:** Task `db:generate` runs migrate diff checks + `sqlc generate`; CI fails if `db/` or generated Go is dirty.

### 9. Free-tier pause vs migrate apply

**Smell:** Project paused → apply fails mid-release.

**Mitigation:** Resume project before migrate; document in runbook. Not an Atlas bug.

### 10. Floci confusion

**Smell:** Expecting Floci to host Postgres or run Atlas.

**Mitigation:** Floci = AWS API/S3/Lambda/…; Postgres is Compose or Supabase. Atlas talks `postgres://` only.

## Blockers / hard constraints

| # | Blocker | Severity | Notes |
|---|---------|----------|--------|
| B1 | **Transaction pooler (`:6543`) breaks `migrate apply`** | High | Advisory lock is session-scoped; Atlas docs: **do not use**. Use **direct `:5432`** or session pooler as IPv4 fallback. |
| B2 | **GitHub-hosted CI often IPv4-only** | High | Direct Supabase host is IPv6 (unless IPv4 add-on). CI must use **session pooler** URL or IPv4 add-on / self-hosted runner. |
| B3 | **Atlas Pro: `docker` block + grant management** | Medium | Official Supabase guide marks custom `docker "postgres"` baseline and permission management as **Pro / `atlas login`**. OSS can still use `dev = "docker://postgres/16/dev"` and put `REVOKE`/`GRANT` in **SQL migration files**. Validate OSS path early in a spike. |
| B4 | **Supabase-shaped extensions on stock image** | Medium | If schema needs `pgcrypto`/`uuid-ossp` in `extensions`, either create them in a migration (if allowed on Free) or baseline script — stock image may lack Supabase-only extensions (`pg_graphql`, etc.). Prefer **no** dependency on those for app schema. |
| B5 | **Existing SQLite / DDB catalog data** | Medium (product) | Atlas does not migrate SQLite→Postgres. Need a one-shot data import task or dual-run cutover — separate grooming. |
| B6 | **Concurrent index / long locks on Free Nano** | Low–Med | Large `CREATE INDEX` on busy table can hurt. Use `CREATE INDEX CONCURRENTLY` outside a transaction (goose-style annotation; Atlas supports non-transactional statements — verify in spike). |
| B7 | **Declarative `schema apply` to prod** | Process | Prefer **versioned migrations** only for prod. Declarative apply is fine for throwaway local reset, dangerous for Free prod with data. |
| B8 | **Two envs, one migration dir** | Process | `local` and `supabase` must apply the **same** `migrations/` history. Never fork dirs per env. |

## Spike checklist (before committing the stack)

1. Install Atlas OSS; `atlas version` without login.
2. Compose Postgres; `env "local"` with multi-file HCL (`experiment` + `experiment_instance` only).
3. `atlas migrate diff` + `migrate apply` against Compose.
4. Create empty Supabase Free project; apply **same** migrations via **direct** URL (or session pooler from IPv4 machine).
5. Confirm `atlas` revisions schema exists; `public` has only app tables.
6. `REVOKE ALL ON ALL TABLES IN SCHEMA public FROM anon, authenticated;` (SQL migration) and prove PostgREST cannot read.
7. Confirm CI story: session pooler secret for `migrate apply` on GitHub Actions.
8. Optional: sqlc one query `ListExperiments` end-to-end.

**Exit for spike:** steps 1–7 green on OSS without Atlas Cloud token; document any command that forced Pro.

## Feasibility result (2026-10-23, local)

Ran against Atlas **v1.3.6** canary OSS (`~/.local/bin/atlas`), disposable `postgres:16-alpine`, multi-file HCL under `db/`.

| Check | Result |
|-------|--------|
| Install without sudo | OK — download binary to `~/.local/bin` (installer sudo to `/usr/local` failed on this host) |
| Multi-file `fileset("schema/**/*.pg.hcl")` | **OK** — `experiment` + `experiment_instance` merged |
| `atlas schema inspect --url env://src` without login | **OK** (prints Pro upsell for triggers/functions; tables/FKs fine) |
| `migrate diff` with `dev = docker://postgres/16/dev?search_path=public` | **OK** after fix (see below) |
| `migrate apply` + `revisions_schema = "atlas"` | **OK** — tables in `public`, history in `atlas.atlas_schema_revisions` |
| Second apply (noop) | **OK** |
| Second `migrate diff` (add column) + apply | **OK** |
| `docker "postgres" "dev" { ... }` block without login | **Blocked** — `Error: requires 'atlas login'` (Pro), as documented |
| Supabase Free apply / CI IPv4 / PostgREST revoke | **Not run** (needs project + network) |

**Diff failure we hit (documented smell):** with `search_path=public` / `schemas = ["public"]`, a `schema "public" { comment = "…" }` desired state fails:

`modify schema "public" is not allowed when migration plan is scoped to one schema`

Use a bare `schema "public" {}` (no comment) when scoped to one schema.

**Verdict:** Atlas OSS is **feasible** for this control-plane schema (HCL multi-file, versioned migrations, local Postgres). Do **not** depend on Pro `docker` blocks or grant DSL — use stock `docker://postgres/…` for `dev` and SQL migrations for `REVOKE`/`GRANT`. Remaining risk is **Supabase connectivity (B1/B2)** and **Data API lockdown (smell 3)**, not Atlas core.

## Non-goals

- Putting parquet / TI / `report.json` in Postgres.
- Replacing Floci or experiment job runners with Atlas.
- Supabase Edge Functions as the Experiment API.
- Managing `auth` / `storage` schemas with Atlas.

## Acceptance (tooling track)

- Desired schema is multi-file HCL under `db/schema/`.
- Every prod schema change is a reviewed file under `db/migrations/` with updated `atlas.sum`.
- Local Compose and Supabase Free share one migration history.
- App tables are not anonymously readable via Supabase Data API.
- Task/docs: how to diff, apply local, apply supabase, and which URL to use.

## Open questions

- Atlas OSS vs Pro: does the team accept SQL-only grants (no Pro), or budget Pro for `docker` baseline + grant blocks?
- IPv4 add-on on Free vs session pooler for CI only?
- One Postgres schema `public` vs app schema `app` (cleaner revoke surface; slightly more URL config)?
- Baseline existing empty Supabase vs apply from migration 1 on a fresh project only?

## References

- [Atlas Supabase guide](https://atlasgo.io/guides/postgres/supabase)
- [HCL directory structure](https://atlasgo.io/faq/hcl-directory-structure)
- [Supabase connection methods](https://supabase.com/docs/guides/database/connecting-to-postgres)
- Prior: `docs/grooming/deploy-supabase.md` (whole-platform “don’t host jobs here”); this doc is **DB schema tooling only** for the control plane.
