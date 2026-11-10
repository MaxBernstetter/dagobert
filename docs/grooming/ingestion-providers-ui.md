# Connectors page — provider activation

UI at `/connectors` (Ingestion section) for configuring plugged-in market-data providers. Activate runs setup; only ready providers appear in Experiments.

Builds on [ingest-list-assets](ingest-list-assets.md) and [ingest-provider-facade](ingest-provider-facade.md).

## Locked decisions

| # | Choice |
|---|--------|
| State | **Postgres**: activation, status, progress, non-secret config (`provider`) and the setup snapshot (`provider_pair`). Secrets are in AWS Secrets Manager (file under `DATA_DIR` with `SECRETS_BACKEND=file`) and are never sent to the browser |
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
    secrets.json          # SECRETS_BACKEND=file only, mode 0600; never returned by API, never in Postgres

Secrets Manager (default in the API Lambda and with AWS_ENDPOINT_URL or SSM_PREFIX): secret /dagobert/<env>/providers/<provider-id>, a JSON object of key to value.
```

Postgres (`db/schema/connector`, `db/sqlc/queries/connectors.sql`, `src/experimentapi/pgcatalog/connectors.go`):

- `provider` (one row per provider): `activated`, `status`, `error`, `progress_label`/`progress_done`/`progress_total`, `config` (jsonb, SQL NULL when never saved), `updated_at_ms`.
- `provider_pair` (provider_id, base, quote, ordinal): the snapshot of a successful setup, in upstream order.

The connector routes need `DATABASE_URL`; without it they answer 503 with a message. The orchestrator uses the same `pgcatalog` connection as the experiment catalog (its own pool).

## Setup (Binance)

On Activate / Retry:

1. Persist submitted config/secrets.
2. `status=setting_up`, progress label e.g. “Fetching markets”.
3. Call adapter `ListPairs` via orchestrator.
4. Fetch the markets from the exchange (the in-memory pairs cache is bypassed; progress becomes the pair count). Activation downloads no candles: candles are ingested only when an experiment or the pipeline needs them.
5. One transaction: `status=ready`, `activated=true` and the snapshot in `provider_pair` (replacing the previous one). On error: `status=failed`, `activated=false`, the previous snapshot is kept.

Deactivate: mark inactive; keep the snapshot and config in Postgres; remove from Experiments immediately.

### Base URL and secrets

- Effective Binance base URL: the saved config `baseUrl` when set, else `BINANCE_BASE_URL`, else `https://data-api.binance.vision` (`effectiveBaseURL` in `provider_settings.go`). Setup, the pairs listing and the job containers (ingest, backfill, experiment runs) all use this value.
- Saving a new base URL does not flip an activated provider. Its snapshot stays until the next activation (Refresh assets), which reads the markets from the new URL.
- Secrets: Secrets Manager secret `/<project>/<env>/providers/<id>` (see `src/experimentapi/providers/secrets.go`), or `DATA_DIR/providers/secrets.json` (mode 0600, written atomically) with `SECRETS_BACKEND=file`. Only key names come back (`secretKeysConfigured`); the values are never in any response. Postgres holds no secrets.
- Moving an existing `secrets.json` to Secrets Manager (once, per provider id; the JSON value is the provider's object):
  `aws secretsmanager create-secret --name /dagobert/<env>/providers/binance --secret-string "$(jq -c '.providers.binance' DATA_DIR/providers/secrets.json)"` (use `put-secret-value --secret-id ...` if it exists). On Floci add `--endpoint-url http://127.0.0.1:11566`.
- Nothing is copied from the old file catalog. Config saved before the move is gone from Postgres: set the base URL again if it was not the default.

Upgrade caveat: the earlier file-based catalog (`DATA_DIR/providers/catalog.json`, `providers/binance/pairs.json`) is not imported. Existing installs re-activate Binance in Connectors once after the upgrade.

## API

The Connectors page calls `ProviderService` (Connect, `proto/tradingcopilot/v1/provider_service.proto`). `ActivateProvider` runs the setup before it answers; the REST routes below are deprecated and answer the same data (the REST activate answers 202 and the setup runs in the background).


| Method | Path | Role |
|--------|------|------|
| `GET` | `/api/v1/providers` | Plugged-in list + catalog status/config (redacted) |
| `GET` | `/api/v1/providers/ready` | Ready providers only (Experiments) |
| `GET` | `/api/v1/providers/{id}` | Detail + progress for polling |
| `PATCH` | `/api/v1/providers/{id}` | Save config and secrets: body `{config, secrets}`; only declared fields (others 400, unknown provider 404); status is not changed; the response is the provider view; secret values are never returned |
| `POST` | `/api/v1/providers/{id}/activate` | Start setup |
| `POST` | `/api/v1/providers/{id}/deactivate` | Drop from ready set |
| `GET` | `/api/v1/providers/{id}/pairs` | Stored snapshot when the provider is ready, otherwise the markets fetched live (nothing is stored); feeds Experiments market multi-select |

Not implemented (the list above is the plan): `GET /api/v1/providers/ready`. The readiness filter for Experiments is the `status` in `GET /api/v1/providers`.

## Acceptance

- Activate Binance from `/connectors` (Ingestion), see running/progress, then see it in Experiments.
- Before activation (or after deactivate), absent from Experiments.
- Secret values never appear in network responses or client bundles.
- Setup failure surfaces error + Retry.
- Reload restores the provider state from Postgres (re-activation is needed once after the move from the file catalog).

## Out of scope

- New exchange adapters beyond descriptors
- Candle backfill UI on this page
- Friendly asset names
