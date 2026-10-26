# Frontend

Vite + React + TypeScript single-page app. It talks only to the Go Experiment API and never reads `./data`, SQLite or Parquet. It does not start jobs.

## API

`VITE_EXPERIMENT_API_URL` is the origin the browser prefixes on `/api/v1/...`. Leave it empty for same-origin `/api`. `npm run dev` and `vite preview` proxy `/api` to `VITE_DEV_API_PROXY`, else `VITE_EXPERIMENT_API_URL`, else `http://127.0.0.1:8090` (the orchestrator, which serves the API when `DATABASE_URL` is set). See `vite.config.ts`.

JSON field names are camelCase (protojson). The API client is `src/api.ts`.

## Commands

```bash
cd src/frontend
npm install
npm run dev      # Vite dev server on :3000; the Experiment API must be reachable
npm run build    # typecheck (tsc) and build to dist/; this is the check gate
```

In Docker, `task up` (dev server, bind-mounted source) or `task dev` (with compose watch) publishes the SPA on `WEB_PORT` (default 3001). `task up:prod` serves the built `dist/` instead. The container proxies `/api` to `http://orchestrator:8090`. There is no `./data` volume. Set `VITE_EXPERIMENT_API_URL` at image build time only when the browser should call the API origin directly (the API must then allow this site's CORS).

## Layout (partly legacy)

The app was migrated from Next.js and the old layout is still in use. Only `src/` is new code.

- `src/` holds the Vite entry points, `api.ts`, `wikiCatalog.ts` and `src/shims/`.
- `app/`, `components/`, `lib/`, `hooks/` and `types/` sit next to `src/`, not inside it. They are the old Next-style tree and are imported through the `@` alias (`vite.config.ts` maps `@` to the frontend root).
- `src/shims/` (`next-link.tsx`, `next-navigation.ts`, `server-only.ts`) stand in for `next/link`, `next/navigation` and `server-only` so that legacy files compile without Next.js. Next.js itself is not a dependency.

New code should go under `src/`. Moving the legacy folders into `src/` and dropping the shims is open cleanup.
