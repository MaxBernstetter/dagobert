import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test";

// Experiment API over Connect unary JSON (lowerCamelCase fields, int64 as strings).
const apiBase = (process.env.E2E_API_BASE ?? process.env.E2E_BASE_URL ?? "").replace(/\/$/, "");
const SERVICE = "/tradingcopilot.v1.ExperimentService";

const COMPLETE_TIMEOUT_MS = 10 * 60_000;

type Date3 = { year: number; month: number; day: number };
type GridSetup = {
  name: string;
  from: Date3;
  to: Date3;
  /** Enum values to keep per axis, as shown in the multi-select (e.g. "3h"). Undefined keeps all. */
  fast?: string[];
  slow?: string[];
  /** Markets to select exactly, as labelled in the picker, e.g. "BTC · USDT". */
  markets: string[];
  parallelism: number;
};

type ExperimentJson = {
  id: string;
  status: string;
  progressStage: number;
  progressStages: number;
  progressInstanceDone: number;
  progressInstanceTotal: number;
  error?: string;
};

type InstanceJson = {
  id: string;
  label: string;
  status: string;
  metrics?: { maxDrawdown?: number; cagr?: number };
};

type StatusResponse = {
  experiment: ExperimentJson;
  job?: { running?: boolean; error?: string };
  total: string;
  instances: InstanceJson[];
};

async function rpc<T>(request: APIRequestContext, method: string, body: unknown): Promise<T> {
  const res = await request.post(`${apiBase}${SERVICE}/${method}`, {
    headers: { "Content-Type": "application/json" },
    data: body,
  });
  if (!res.ok()) {
    throw new Error(`${method} failed with HTTP ${res.status()}: ${await res.text()}`);
  }
  return (await res.json()) as T;
}

/** Status badge text in the open experiment, e.g. "Completed" or "Running · 40% · 2/4 · Instance simulation". */
async function statusText(page: Page): Promise<string> {
  return ((await page.getByTestId("experiment-status").textContent()) ?? "").trim();
}

async function destructiveTexts(page: Page): Promise<string> {
  const texts = await page.locator("p.text-destructive").allTextContents();
  return texts.map((t) => t.trim()).filter(Boolean).join(" | ");
}

/** Polls the status badge until it reaches a terminal state; returns it. */
async function waitForTerminalStatus(page: Page): Promise<string> {
  let last = "";
  await expect
    .poll(
      async () => {
        last = await statusText(page);
        return /^(Completed|Failed|Cancelled)$/.test(last) ? last : "";
      },
      { timeout: COMPLETE_TIMEOUT_MS, intervals: [3_000] },
    )
    .not.toBe("");
  return last;
}

/**
 * Experiments only offer providers whose status is ready. On a fresh stack Binance is inactive, so activate it
 * under Connectors (Tools menu) and wait for the asset catalog to load. No-op when it is already ready.
 */
async function ensureBinanceReady(page: Page) {
  await page.goto("/");
  await page.getByRole("button", { name: "Tools" }).click();
  await page.getByRole("link", { name: "Connectors" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Connectors" })).toBeVisible();
  const binance = page.getByRole("button", { name: /^Binance/ }).first();
  await binance.click();
  const activate = page.getByRole("button", { name: "Activate", exact: true });
  if (await activate.isVisible()) {
    await activate.click();
  }
  await expect(page.getByText("Ready", { exact: true }).first()).toBeVisible({ timeout: 5 * 60_000 });
  await page.getByRole("banner").getByRole("link", { name: "Experiment Suite" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Experiments" })).toBeVisible();
}

/** Opens a calendar date picker (From or To) and selects the given day. */
async function pickDate(page: Page, buttonName: string, date: Date3) {
  await page.getByRole("button", { name: buttonName }).click();
  const calendar = page.getByRole("dialog").filter({ has: page.getByRole("grid") });
  await calendar.getByRole("combobox", { name: /month/i }).selectOption({ value: String(date.month - 1) });
  await calendar.getByRole("combobox", { name: /year/i }).selectOption({ value: String(date.year) });
  await calendar.locator("button[data-day]").filter({ hasText: new RegExp(`^${date.day}$`) }).first().click();
  await expect(calendar).toHaveCount(0);
}

/** Sets one GRID axis card (fast or slow) to exactly the listed values, e.g. ["3h", "6h"]. */
async function setGridAxis(page: Page, key: "fastHr" | "slowHr", keep: string[] | undefined) {
  if (!keep) return;
  await page.getByTestId(`grid-axis-${key}`).getByRole("button").first().click();
  const options = page.getByRole("checkbox", { name: /^\d+h$/ });
  await expect(options.first()).toBeVisible();
  const names = await options.evaluateAll((els) => els.map((el) => el.closest("label")?.textContent?.trim() ?? ""));
  for (let i = 0; i < names.length; i++) {
    const box = options.nth(i);
    if ((await box.isChecked()) !== keep.includes(names[i])) await box.click();
  }
  await page.keyboard.press("Escape");
  // Wait for the popover to unmount, so the next axis does not read this one's checkboxes.
  await expect(options).toHaveCount(0);
}

/**
 * Sets the Markets picker to exactly the given pairs (e.g. ["BTC · USDT"]). The catalog has about 1,400
 * pairs, so the search box narrows the list before each checkbox is toggled.
 */
async function setMarkets(page: Page, pairs: string[]) {
  await page.locator("label", { hasText: "Markets" }).getByRole("button").first().click();
  const search = page.getByPlaceholder("Search…");
  for (const pair of pairs) {
    await search.fill(pair);
    const box = page.getByRole("checkbox", { name: pair, exact: true });
    await expect(box).toHaveCount(1);
    if (!(await box.isChecked())) await box.click();
  }
  // Drop any other pair the picker preselected (the default is the first preferred bases).
  await search.fill("");
  const checked = page.getByRole("checkbox", { checked: true });
  const extra = await checked.evaluateAll((els) => els.map((el) => el.closest("label")?.textContent?.trim() ?? ""));
  for (const label of extra) {
    if (pairs.includes(label)) continue;
    await search.fill(label);
    await page.getByRole("checkbox", { name: label, exact: true }).click();
  }
  await search.fill("");
  await page.keyboard.press("Escape");
}

/** Fills the New experiment dialog in GRID mode and creates it; returns the grid size shown on the button. */
async function createGridExperiment(page: Page, setup: GridSetup): Promise<number> {
  await page.getByRole("button", { name: "New experiment", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New experiment" });
  await expect(dialog).toBeVisible();

  await dialog.getByLabel("Name", { exact: true }).fill(setup.name);
  await dialog.getByRole("button", { name: "GRID", exact: true }).click();
  await dialog.locator("#market-provider").selectOption("binance");
  await setMarkets(page, setup.markets);
  await pickDate(page, "From (UTC)", setup.from);
  await pickDate(page, "To (UTC, inclusive)", setup.to);
  await setGridAxis(page, "fastHr", setup.fast);
  await setGridAxis(page, "slowHr", setup.slow);
  await dialog.getByLabel(/^Parallel processes/).fill(String(setup.parallelism));

  const create = dialog.getByRole("button", { name: /^Create \(\d+\)$/ });
  const match = /\((\d+)\)/.exec((await create.textContent()) ?? "");
  const size = match ? Number(match[1]) : 0;
  await create.click();
  await expect(page.getByRole("heading", { level: 1, name: setup.name })).toBeVisible({ timeout: 30_000 });
  return size;
}

/** Column index of a table header whose text matches the pattern. */
async function columnIndex(table: Locator, pattern: RegExp): Promise<number> {
  const headers = await table.locator("thead th").allTextContents();
  const index = headers.findIndex((h) => pattern.test(h.trim()));
  expect(index, `column ${pattern} in ${JSON.stringify(headers)}`).toBeGreaterThanOrEqual(0);
  return index;
}

const NUMERIC_CELL = /^[+-]?\d+(\.\d+)?(%|pp|×)?$/;

test.describe("experiments against Floci", () => {
  test("creates a ma-cross grid, runs it to completion and checks the outcome", async ({ page, request }) => {
    test.setTimeout(COMPLETE_TIMEOUT_MS + 5 * 60_000);
    const name = `e2e ma-cross ${new Date().toISOString().replace(/[:.]/g, "-")}`;

    await ensureBinanceReady(page);
    const size = await createGridExperiment(page, {
      name,
      from: { year: 2024, month: 3, day: 1 },
      to: { year: 2024, month: 3, day: 8 },
      fast: ["3h", "6h"],
      slow: ["12h", "24h"],
      markets: ["BTC · USDT"],
      parallelism: 2,
    });
    expect(size, "grid size before create").toBe(4);

    await page.getByRole("button", { name: "Run experiment", exact: true }).click();
    const terminal = await waitForTerminalStatus(page);
    if (terminal !== "Completed") {
      const detail = await destructiveTexts(page);
      throw new Error(`experiment ended ${terminal}${detail ? `: ${detail}` : ""}`);
    }
    await expect(page.getByText(/· 4 instances\b/)).toBeVisible();

    // Instance table: one row per grid point, KPI cells numeric.
    const table = page.getByRole("table");
    const rows = table.locator("tbody tr");
    await expect(rows).toHaveCount(size);
    const ddColumn = await columnIndex(table, /^max dd$|^max drawdown$/i);
    const cagrColumn = await columnIndex(table, /^cagr$/i);
    for (let i = 0; i < size; i++) {
      for (const col of [ddColumn, cagrColumn]) {
        const text = ((await rows.nth(i).locator("td").nth(col).textContent()) ?? "").trim();
        expect(text, `row ${i} column ${col}`).toMatch(NUMERIC_CELL);
        expect(text).not.toMatch(/NaN/);
      }
    }

    // API cross-check: completed, progress complete, invariants on every instance.
    const status = await rpc<StatusResponse>(request, "GetExperimentStatus", { id: await experimentIdByName(request, name), limit: 200 });
    expect(status.experiment.status).toBe("completed");
    expect(status.experiment.progressInstanceTotal).toBe(size);
    expect(status.experiment.progressInstanceDone).toBe(size);
    expect(status.experiment.progressStage).toBe(status.experiment.progressStages);
    expect(Number(status.total)).toBe(size);
    expect(status.instances).toHaveLength(size);

    let withFills = 0;
    for (const inst of status.instances) {
      expect(inst.status, inst.label).toBe("completed");
      // Drawdown is stored and displayed as a positive magnitude (0.31 shows as 31.1%).
      const dd = inst.metrics?.maxDrawdown;
      expect(dd, `${inst.label} max drawdown`).toBeDefined();
      expect(dd!).toBeGreaterThanOrEqual(0);
      expect(dd!).toBeLessThanOrEqual(1);

      const activity = await rpc<{ rows: { kind: string }[] }>(request, "GetInstanceActivity", {
        experimentId: status.experiment.id,
        instanceId: inst.id,
      });
      const fills = activity.rows.filter((r) => r.kind === "fill").length;
      if (fills > 0) withFills++;

      const kpis = await rpc<{ kpis: { comparison?: { startCash: number; strategyReturn: number } } }>(
        request,
        "GetInstanceKpis",
        { experimentId: status.experiment.id, instanceId: inst.id },
      );
      const cmp = kpis.kpis.comparison;
      expect(cmp, `${inst.label} comparison`).toBeDefined();
      const finalEquity = cmp!.startCash * (1 + cmp!.strategyReturn);
      expect(finalEquity, `${inst.label} final equity`).toBeGreaterThan(0);
    }
    expect(withFills, "instances with at least one fill").toBeGreaterThan(0);

    // Instance detail: the charts section renders without an error.
    const filled = status.instances.find((i) => i.status === "completed");
    expect(filled).toBeDefined();
    await page.getByRole("row", { name: new RegExp(escapeRegExp(filled!.label)) }).click();
    await expect(page.getByText("Charts & decisions", { exact: true })).toBeVisible({ timeout: 60_000 });
    await expect(page.getByText(/BigInt|Cannot convert|TypeError|NaN/)).toHaveCount(0);
    await expect(page.locator("p.text-destructive")).toHaveCount(0);
  });

  test("cancelling a running experiment leaves it cancelled", async ({ page, request }) => {
    test.setTimeout(COMPLETE_TIMEOUT_MS);
    const name = `e2e cancel ${new Date().toISOString().replace(/[:.]/g, "-")}`;

    // Default grid (all fast/slow windows, 10 valid pairs) over a range not fetched before, so the run is still going when cancelled.
    await ensureBinanceReady(page);
    await createGridExperiment(page, {
      name,
      from: { year: 2023, month: 1, day: 1 },
      to: { year: 2023, month: 6, day: 30 },
      markets: ["BTC · USDT"],
      parallelism: 2,
    });

    await page.getByRole("button", { name: "Run experiment", exact: true }).click();
    const cancel = page.getByRole("button", { name: "Cancel run", exact: true });
    await expect(cancel).toBeVisible({ timeout: COMPLETE_TIMEOUT_MS });
    await cancel.click();

    await expect
      .poll(() => statusText(page), { timeout: 60_000, intervals: [1_000] })
      .toBe("Cancelled");

    // It must stay cancelled: no later worker or retry may flip it to completed or failed.
    await page.waitForTimeout(20_000);
    await expect(page.getByTestId("experiment-status")).toHaveText("Cancelled");
    const status = await rpc<StatusResponse>(request, "GetExperimentStatus", {
      id: await experimentIdByName(request, name),
      limit: 1,
    });
    expect(status.experiment.status).toBe("cancelled");
  });
});

/** Finds the experiment id by its name through ListExperiments (names are unique per run). */
async function experimentIdByName(request: APIRequestContext, name: string): Promise<string> {
  const list = await rpc<{ experiments: { id: string; name: string }[] }>(request, "ListExperiments", {});
  const hit = list.experiments.find((e) => e.name === name);
  if (!hit) throw new Error(`experiment "${name}" not found via ListExperiments`);
  return hit.id;
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
