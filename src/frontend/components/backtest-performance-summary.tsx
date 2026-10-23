"use client";

import { formatStrategyKpiValue, type StrategyKpiComparison } from "@/lib/strategy-kpi-shared";
import { cn } from "@/lib/utils";

export function BacktestPerformanceSummary({
  comparison,
  className,
}: {
  comparison: StrategyKpiComparison | undefined;
  className?: string;
}) {
  if (!comparison) {
    return null;
  }
  const active = comparison.strategyReturn - comparison.buyHoldReturn;
  const strat = formatStrategyKpiValue("percent", comparison.strategyReturn);
  const bh = formatStrategyKpiValue("percent", comparison.buyHoldReturn);
  const vs = formatStrategyKpiValue("percent-pts", active);

  return (
    <p className={cn("text-sm text-muted-foreground", className)}>
      <span className="text-foreground font-medium">Strategy {strat}</span>
      {" · "}
      Buy &amp; hold {bh}
      {" · "}
      <span title="Strategy total return minus buy-and-hold on the same window and starting quote cash">
        vs B&amp;H {vs}
      </span>
      <span className="block text-xs mt-1">
        Benchmark: deploy all {comparison.startCash.toLocaleString()} USDC into BTC at the first bar close;
        strategy uses your allocation settings.
      </span>
    </p>
  );
}
