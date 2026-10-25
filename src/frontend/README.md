# Experiment SPA

Vite + React static UI. The browser calls the Go Experiment API and does not mount `DATA_DIR`, use `better-sqlite3`, or read SQLite. The Next app in `src/frontend` stays until cutover.

## API

`VITE_EXPERIMENT_API_URL` is the origin the browser prefixes on `/api/v1/...`. Leave it empty to use same-origin `/api`. `npm run dev` and `vite preview` proxy `/api` to `VITE_DEV_API_PROXY`, or to `VITE_EXPERIMENT_API_URL` when the proxy variable is unset, or to `http://127.0.0.1:8090`.

JSON field names match the Go API protojson (camelCase): `experiments`, `instances`, `nextCursor`, `fromMs`, `toMs`, `initialUsdc`, `listSymbols`, `maCross`, and chart `bars` / `indicators`.

The chart panel requests:

`GET /api/v1/experiments/{id}/instances/{instanceId}/chart-series?from=&to=`

`from` and `to` are unix seconds.

## Run on the host

The Experiment API must already be listening (orchestrator default `127.0.0.1:8090`).

```bash
cd src/frontend
cp .env.example .env   # optional
npm install
npm run dev
```

Open http://localhost:3000. `npm run typecheck` and `npm run build` check and emit `dist/`.

## Compose

`docker compose up` and `task up` publish this SPA on `WEB_PORT`. The frontend service has no `DATA_DIR` and no `./data` volume. `src/frontend` stays on disk and is not mounted.

```bash
docker compose up --build frontend
```

Inside Compose the dev server proxies `/api` to `http://orchestrator:8090` (`VITE_DEV_API_PROXY`). The browser stays on `WEB_PORT` and never sees the data directory. For a static image, change `target: dev` in `compose.spa.yaml` to `target: prod` (`vite preview` of `dist/`). Set `VITE_EXPERIMENT_API_URL` at image build time only when the browser should call the API origin directly; that origin must allow this site's CORS.
