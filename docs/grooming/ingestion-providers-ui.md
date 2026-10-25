# Connectors page — provider activation

UI at `/connectors` (Ingestion section) for configuring plugged-in market-data providers. Activate runs setup; only ready providers appear in Experiments. The Trading section lists Bybit credentials from `.secrets` (masked).

Builds on [ingest-list-assets](ingest-list-assets.md) and [ingest-provider-facade](ingest-provider-facade.md).

## Locked decisions

| # | Choice |
|---|--------|
| Secrets / config | **`DATA_DIR` catalog** (Next server-side read/write only; never send secret values to the browser) |
| Activate | **Activate = setup**; provider becomes selectable in Experiments **only after setup succeeds** |
| Feedback | Setup shows **status** (`inactive` / `setting_up` / `ready` / `failed`) and **progress** when applicable |
| First provider | **Binance** (public; no API key required today; schema supports secret fields for later providers) |

## Product model

- **Plugged in** = compiled/registered adapters (code registry), always listed on Ingestion.
- **Ready** = setup finished successfully; appears in Experiment Suite provider dropdown.
- **Inactive** = never activated, or deactivated (config may remain; not selectable in Experiments).
- **Failed** = last setup failed; stay off Experiments; allow retry Activate.

## DATA_DIR layout

```text
DATA_DIR/
  providers/
    catalog.json          # activation + non-secret config + status/progress
    secrets.json          # mode 0600; API keys etc. — never returned by API
    binance/
      pairs.json          # durable ListPairs snapshot written by setup
```

## Setup (Binance)

On Activate / Retry:

1. Persist submitted config/secrets.
2. `status=setting_up`, progress label e.g. “Fetching markets”.
3. Call adapter `ListPairs` via orchestrator.
4. Write `providers/binance/pairs.json`; update progress to pair count.
5. `status=ready`, `activated=true`. On error: `status=failed`, `activated=false`.

Deactivate: mark inactive; keep pairs+config on disk; remove from Experiments immediately.

## API

| Method | Path | Role |
|--------|------|------|
| `GET` | `/api/v1/providers` | Plugged-in list + catalog status/config (redacted) |
| `GET` | `/api/v1/providers/ready` | Ready providers only (Experiments) |
| `GET` | `/api/v1/providers/{id}` | Detail + progress for polling |
| `PATCH` | `/api/v1/providers/{id}` | Upsert public config + optional secrets |
| `POST` | `/api/v1/providers/{id}/activate` | Start setup |
| `POST` | `/api/v1/providers/{id}/deactivate` | Drop from ready set |
| `GET` | `/api/v1/providers/{id}/pairs` | Disk snapshot only (provider must be ready); feeds Experiments market multi-select |

## Acceptance

- Activate Binance from `/connectors` (Ingestion), see running/progress, then see it in Experiments.
- Before activation (or after deactivate), absent from Experiments.
- Secret values never appear in network responses or client bundles.
- Setup failure surfaces error + Retry.
- Reload restores catalog state from `DATA_DIR`.

## Out of scope

- New exchange adapters beyond descriptors
- Migrating Bybit execution secrets into this catalog
- Candle backfill UI on this page
- Friendly asset names
