# Frontend

Vite + React + TypeScript single-page app. It talks only to the Go Experiment API and never reads `./data`, SQLite or Parquet. It does not start jobs.

## API

`VITE_EXPERIMENT_API_URL` is the origin the browser prefixes on `/api/v1/...`. Leave it empty for same-origin `/api`. `npm run dev` and `vite preview` proxy `/api` to `VITE_DEV_API_PROXY`, else `VITE_EXPERIMENT_API_URL`, else `http://127.0.0.1:11090` (the orchestrator, which serves the API when `DATABASE_URL` is set). See `vite.config.ts`.

JSON field names are camelCase (protojson). The API client is `src/api.ts`.

## Commands

```bash
cd src/frontend
npm install
npm run dev      # Vite dev server on :3000; the Experiment API must be reachable
npm run build    # typecheck (tsc) and build to dist/; this is the check gate
```

In Docker, `task up` (dev server, bind-mounted source) or `task dev` (with compose watch) publishes the SPA on `WEB_PORT` (default 11001). `task up:prod` serves the built `dist/` instead. The container proxies `/api` to `http://orchestrator:8090`. There is no `./data` volume. Set `VITE_EXPERIMENT_API_URL` at image build time only when the browser should call the API origin directly (the API must then allow this site's CORS).

## Layout

Everything lives under `src/`; the `@` alias (`vite.config.ts`, `tsconfig.json`) maps to `src/`.

- `main.tsx`, `App.tsx`: entry point and routes (react-router).
- `api.ts`, `wikiCatalog.ts`, `WikiPages.tsx`, `ChartPanel.tsx`: API client, wiki catalog and pages, chart.
- Instance charts (`components/strategy-charts.tsx`, `time-chart.tsx`) load `chart-series` with `maxPoints` from `lib/chart-series-cache.ts` (`chartMaxPoints`, `chartSeriesWindowUrl`). Zoom and range presets refetch their window with the same budget; buy and hold uses the full-range series. The data is cached by URL. Rendering speed is not covered by a test suite.
- `components/`: UI (`components/ui/` holds the shadcn primitives; `link.tsx` wraps the router link).
- `lib/`: helpers and API types; `lib/gen/` is generated protobuf-es output (`task proto:gen`).
- `hooks/`, `types/`: React hooks and shared types.
- `globals.css`, `styles.css`: styles.
