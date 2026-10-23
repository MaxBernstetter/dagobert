import "server-only";

import fs from "fs";
import path from "path";
import matter from "gray-matter";

import { defaultStrategyKpiDefs, type StrategyKpiMeta } from "@/lib/strategy-kpi-shared";

export type { StrategyKpiMeta, StrategyKpiResult, StrategyKpiSet } from "@/lib/strategy-kpi-shared";
export { formatStrategyKpiValue } from "@/lib/strategy-kpi-shared";

export type StrategyKpiEntry = {
  meta: StrategyKpiMeta;
  content: string;
};

const KPI_SLUG_ORDER = ["cagr", "max-drawdown", "sharpe", "active-return", "sortino"];

function repoRoot(): string {
  if (process.env.REPO_ROOT) {
    return process.env.REPO_ROOT;
  }
  return path.resolve(process.cwd(), "..", "..");
}

export function strategyKpiDir(): string {
  const wikiPlaybooks = process.env.WIKI_PLAYBOOKS_DIR?.trim();
  const candidates = [
    process.env.STRATEGY_KPI_DIR?.trim(),
    wikiPlaybooks ? path.join(wikiPlaybooks, "strategy-kpi") : "",
    "/wiki/playbooks/strategy-kpi",
    path.join(repoRoot(), "src", "calculation-signal", "playbooks", "strategy-kpi"),
  ].filter((dir): dir is string => Boolean(dir));
  for (const dir of candidates) {
    if (fs.existsSync(dir)) {
      return dir;
    }
  }
  return path.join(repoRoot(), "src", "calculation-signal", "playbooks", "strategy-kpi");
}

function readEntry(slug: string): StrategyKpiEntry | null {
  const file = path.join(strategyKpiDir(), slug, "README.md");
  if (!fs.existsSync(file)) {
    return null;
  }
  const raw = fs.readFileSync(file, "utf8");
  const { data, content } = matter(raw);
  const meta = data as StrategyKpiMeta;
  if (!meta.slug) {
    meta.slug = slug;
  }
  return { meta, content: content.trim() };
}

export function listStrategyKpis(): StrategyKpiEntry[] {
  const root = strategyKpiDir();
  if (!fs.existsSync(root)) {
    return [];
  }
  const slugs = fs
    .readdirSync(root, { withFileTypes: true })
    .filter((e) => e.isDirectory() && fs.existsSync(path.join(root, e.name, "README.md")))
    .map((e) => e.name);
  const order = new Map(KPI_SLUG_ORDER.map((s, i) => [s, i]));
  return slugs
    .map((slug) => readEntry(slug))
    .filter((e): e is StrategyKpiEntry => e !== null)
    .sort((a, b) => {
      const ai = order.get(a.meta.slug) ?? 99;
      const bi = order.get(b.meta.slug) ?? 99;
      if (ai !== bi) return ai - bi;
      return a.meta.title.localeCompare(b.meta.title);
    });
}

export function getStrategyKpi(slug: string): StrategyKpiEntry | null {
  return readEntry(slug);
}

/** KPI metadata for compute + UI: markdown catalog when present, else built-in registry. */
export function strategyKpiDefsForCompute(): StrategyKpiMeta[] {
  const fromDocs = listStrategyKpis().map((e) => e.meta);
  if (fromDocs.length > 0) {
    return fromDocs;
  }
  return defaultStrategyKpiDefs();
}
