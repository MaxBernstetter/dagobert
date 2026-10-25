import { parse as parseYaml } from "yaml";
import type { StrategyKpiMeta } from "@/lib/strategy-kpi-shared";

type IndicatorSeries = { key: string; definition: Record<string, unknown> };
type WikiMeta = {
  title: string;
  slug: string;
  kind: string;
  unit?: string;
  status?: string;
  symbols?: string[];
  venue?: string;
  series?: IndicatorSeries[];
};
export type WikiEntry = { meta: WikiMeta; content: string };
type StrategyKpiEntry = { meta: StrategyKpiMeta; content: string };

const indicatorFiles = import.meta.glob(
  "../../calculation-technical-indicator/internal/indicators/*/README.md",
  { query: "?raw", import: "default", eager: true },
) as Record<string, string>;

const playbookFiles = import.meta.glob(
  "../../calculation-signal/playbooks/*/DESCRIPTION.md",
  { query: "?raw", import: "default", eager: true },
) as Record<string, string>;

const strategyKpiFiles = import.meta.glob(
  "../../calculation-signal/playbooks/strategy-kpi/*/README.md",
  { query: "?raw", import: "default", eager: true },
) as Record<string, string>;

function slugFrom(path: string): string {
  const parts = path.split("/");
  return parts[parts.length - 2] ?? "";
}

function parseWiki(path: string, raw: string): WikiEntry | null {
  try {
    let data: WikiMeta = { title: "", slug: "", kind: "" };
    let content = raw;
    if (raw.startsWith("---")) {
      const end = raw.indexOf("\n---", 3);
      if (end !== -1) {
        data = (parseYaml(raw.slice(4, end)) ?? {}) as WikiMeta;
        content = raw.slice(end + 4);
      }
    }
    if (!data.slug) data.slug = slugFrom(path);
    if (!data.title) data.title = data.slug;
    if (data.series && !Array.isArray(data.series)) data.series = [];
    return { meta: data, content: content.trim() };
  } catch {
    return null;
  }
}

function sorted(entries: WikiEntry[]): WikiEntry[] {
  return entries.sort((a, b) => a.meta.title.localeCompare(b.meta.title));
}

export function wikiIndicators(): WikiEntry[] {
  return sorted(
    Object.entries(indicatorFiles)
      .filter(([path]) => !path.includes("/types/"))
      .map(([path, raw]) => parseWiki(path, raw))
      .filter((entry): entry is WikiEntry => entry !== null),
  );
}

export function wikiPlaybooks(): WikiEntry[] {
  return sorted(
    Object.entries(playbookFiles)
      .map(([path, raw]) => parseWiki(path, raw))
      .filter((entry): entry is WikiEntry => entry !== null),
  );
}

export function wikiStrategyKpis(): StrategyKpiEntry[] {
  return Object.entries(strategyKpiFiles)
    .map(([path, raw]) => {
      const entry = parseWiki(path, raw);
      if (!entry) return null;
      return { meta: entry.meta as StrategyKpiEntry["meta"], content: entry.content };
    })
    .filter((entry): entry is StrategyKpiEntry => entry !== null)
    .sort((a, b) => a.meta.title.localeCompare(b.meta.title));
}

export function indicatorSeries(entry: WikiEntry | undefined): IndicatorSeries[] {
  return [...(entry?.meta.series ?? [])].sort((a, b) => a.key.localeCompare(b.key));
}
