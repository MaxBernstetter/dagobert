/** Layer-1 indicators to plot for a deployed playbook (matches calculation-signal logic). */

export type PlaybookChartIndicator = {
  id: string;
  label: string;
  color: string;
  matches: (def: Record<string, unknown>) => boolean;
};

function slugFromPlaybook(playbook: string): string {
  return playbook.replace(/\.ya?ml$/i, "").trim();
}

export function chartIndicatorsForPlaybook(playbook: string): PlaybookChartIndicator[] {
  switch (slugFromPlaybook(playbook)) {
    case "ma-cross":
      return [
        {
          id: "sma-3h",
          label: "SMA 3h (fast)",
          color: "#38bdf8",
          matches: (d) =>
            d.type === "MovingAverage" && d.method === "sma" && d.time_frame_hr === 3,
        },
        {
          id: "sma-6h",
          label: "SMA 6h (slow)",
          color: "#f59e0b",
          matches: (d) =>
            d.type === "MovingAverage" && d.method === "sma" && d.time_frame_hr === 6,
        },
      ];
    default:
      return [];
  }
}

export function indicatorValuesFromKpiBody(
  body: string,
  indicators: PlaybookChartIndicator[]
): Record<string, number | null> {
  const out: Record<string, number | null> = {};
  for (const ind of indicators) {
    out[ind.id] = null;
  }
  if (indicators.length === 0) return out;
  try {
    const row = JSON.parse(body) as {
      results?: { def: Record<string, unknown> | string; value: number }[];
    };
    for (const result of row.results ?? []) {
      const def =
        typeof result.def === "string"
          ? (JSON.parse(result.def) as Record<string, unknown>)
          : result.def;
      for (const ind of indicators) {
        if (ind.matches(def)) {
          out[ind.id] = result.value;
        }
      }
    }
  } catch {
    /* ignore */
  }
  return out;
}
