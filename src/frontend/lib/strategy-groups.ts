import type { ActivityRow, JournalRow } from "@/lib/read";

export type StrategyGroup = {
  playbook: string;
  title: string;
  rows: ActivityRow[];
  decisionCount: number;
  fillCount: number;
  awaitingExecution: number;
};

export function formatPlaybookTitle(playbook: string): string {
  const base = playbook.replace(/\.ya?ml$/i, "").trim();
  if (!base) return "Unknown strategy";
  return base
    .split(/[-_]+/)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

export function resolvePlaybookTitle(
  playbook: string,
  catalog: Record<string, string>
): string {
  const direct = catalog[playbook];
  if (direct) return direct;
  const slug = playbook.replace(/\.ya?ml$/i, "");
  if (catalog[slug]) return catalog[slug];
  return formatPlaybookTitle(playbook);
}

export function playbookFromOrderLinkId(orderLinkId: string): string {
  const last = orderLinkId.lastIndexOf(":");
  if (last <= 0) return "";
  const rest = orderLinkId.slice(0, last);
  const sep = rest.lastIndexOf(":");
  if (sep <= 0) return rest;
  return rest.slice(0, sep);
}

export function groupActivityByPlaybook(
  rows: ActivityRow[],
  titleCatalog: Record<string, string> = {}
): StrategyGroup[] {
  const map = new Map<string, ActivityRow[]>();
  for (const row of rows) {
    const key = row.playbook || "(no playbook id)";
    const list = map.get(key) ?? [];
    list.push(row);
    map.set(key, list);
  }

  const groups: StrategyGroup[] = [];
  for (const [playbook, list] of map) {
    list.sort((a, b) => b.candleClose - a.candleClose);
    groups.push({
      playbook,
      title: resolvePlaybookTitle(playbook, titleCatalog),
      rows: list,
      decisionCount: list.length,
      fillCount: list.filter((r) => r.kind === "fill").length,
      awaitingExecution: list.filter((r) => r.kind === "not-yet-filled").length,
    });
  }

  groups.sort((a, b) => a.title.localeCompare(b.title));
  return groups;
}

export function journalsForPlaybook(
  playbook: string,
  journals: JournalRow[]
): JournalRow[] {
  return journals.filter((j) => playbookFromOrderLinkId(j.orderLinkId) === playbook);
}
