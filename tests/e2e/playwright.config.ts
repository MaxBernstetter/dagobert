import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.E2E_BASE_URL;
if (!baseURL) {
  throw new Error(
    "E2E_BASE_URL is required, e.g. http://<id>.cloudfront.localhost:11566/ (task e2e sets it from the compiled app URL).",
  );
}

export default defineConfig({
  testDir: ".",
  testMatch: /.*\.spec\.ts/,
  // One run at a time: the tests share the Floci stack, the Binance backfill and the experiment queue.
  workers: 1,
  fullyParallel: false,
  timeout: 20 * 60_000,
  expect: { timeout: 15_000 },
  retries: 1,
  reporter: [["list"], ["html", { open: "never" }]],
  outputDir: "test-results",
  use: {
    baseURL,
    // Every action fails fast with a locator name instead of waiting for the whole test budget.
    actionTimeout: 15_000,
    trace: "retain-on-failure",
    video: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  // The experiment page is laid out for a tall viewport (instance table plus charts); the 720 px default clips it.
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1600, height: 1000 } } }],
});
