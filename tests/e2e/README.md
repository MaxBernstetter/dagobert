# End-to-end tests

Playwright (Chromium) drives the compiled web app against the running Floci stack and checks experiments end to end.

- Run: `task e2e` from the repo root (reads the compiled app URL from Terraform, needs `task up` first).
- Or directly: `cd tests/e2e && npm ci && E2E_BASE_URL=http://<id>.cloudfront.localhost:11566/ npx playwright test`.
- Against the Vite dev server: start it in `src/frontend` with `VITE_DEV_API_PROXY` and `VITE_DEV_API_PREFIX` set, then `E2E_BASE_URL=http://127.0.0.1:3000 npx playwright test`.
- First run in a fresh browser: `npx playwright install --with-deps chromium` (CI) or `npx playwright install chromium`.
- The tests navigate only by clicking from `/` (Floci has no SPA fallback for deep links). Failures keep trace, video and screenshot under `test-results/`; the HTML report is in `playwright-report/`.
- `E2E_API_BASE` overrides the origin for the API cross-check (defaults to `E2E_BASE_URL`).
- Sign-in: the tests log in as the seeded local Cognito user `admin` / `password` (`infra/terraform/cognito.tf`); override with `E2E_USERNAME` and `E2E_PASSWORD`.
