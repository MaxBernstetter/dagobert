/** UI tone for strategy KPI values (higher-is-better unless noted). */
export type KpiTone = "good" | "okay" | "bad" | "neutral";

export function strategyKpiTone(slug: string, value: number): KpiTone {
  if (!Number.isFinite(value)) return "neutral";

  switch (slug) {
    case "cagr":
      if (value >= 0.08) return "good";
      if (value >= 0) return "okay";
      return "bad";
    case "max-drawdown":
      if (value <= 0.15) return "good";
      if (value <= 0.35) return "okay";
      return "bad";
    case "sharpe":
      if (value >= 1) return "good";
      if (value >= 0.4) return "okay";
      return "bad";
    case "sortino":
      if (value >= 1.2) return "good";
      if (value >= 0.5) return "okay";
      return "bad";
    case "active-return":
      if (value > 0.02) return "good";
      if (value >= -0.05) return "okay";
      return "bad";
    default:
      return "neutral";
  }
}

/** Neutral shell for KPI metric tiles (border + background). */
export const kpiTileShellClassName = "border-border bg-muted/40 dark:bg-muted/25";

export function toneTextClassName(tone: KpiTone): string {
  switch (tone) {
    case "good":
      return "text-emerald-600 dark:text-emerald-400";
    case "bad":
      return "text-red-600 dark:text-red-400";
    default:
      return "text-foreground";
  }
}

export function returnTone(value: number): KpiTone {
  if (value >= 0.1) return "good";
  if (value >= 0) return "okay";
  return "bad";
}
