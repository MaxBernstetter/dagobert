"use client";

import {
  BacktestDetailSection,
  BacktestMetricGroup,
  BacktestMetricRow,
} from "@/components/backtest-detail-section";
import {
  decisionsPerMonthStats,
  tradeCountFromFills,
  type BacktestRunStatistics,
} from "@/lib/backtest-period-stats";
import type { BacktestOutcome } from "@/lib/backtest-shared";
import type { BacktestKpiSetState } from "@/hooks/use-backtest-kpi-set";
import { returnTone, strategyKpiTone } from "@/lib/strategy-kpi-tone";
import {
  formatStrategyKpiValue,
  type StrategyKpiComparison,
  type StrategyKpiResult,
} from "@/lib/strategy-kpi-shared";
import { cn } from "@/lib/utils";

function formatDurationMonths(months: number): string {
  if (!Number.isFinite(months) || months <= 0) return "—";
  if (months < 12) return `${months.toFixed(1)} mo`;
  const years = months / 12;
  return `${years.toFixed(2)} yr (${months.toFixed(0)} mo)`;
}

function formatRate(value: number | null | undefined, digits = 1): string {
  if (value == null || !Number.isFinite(value)) return "—";
  return value.toFixed(digits);
}

function kpiValue(results: StrategyKpiResult[], slug: string): number | null {
  const row = results.find((r) => r.def.slug === slug);
  if (row == null || !Number.isFinite(row.value)) return null;
  return row.value;
}

function kpiMeta(results: StrategyKpiResult[], slug: string) {
  return results.find((r) => r.def.slug === slug)?.def;
}

export function BacktestPerformancePanel({
  kpi,
  comparison,
  statistics,
  outcome,
  decisionTimestampsMs,
  loading,
  className,
}: {
  kpi: BacktestKpiSetState;
  comparison: StrategyKpiComparison | undefined;
  statistics: BacktestRunStatistics | undefined;
  outcome: BacktestOutcome | null;
  decisionTimestampsMs?: number[];
  loading?: boolean;
  className?: string;
}) {
  const { results } = kpi;
  const show =
    loading ||
    results.length > 0 ||
    comparison ||
    statistics ||
    outcome;

  if (!show) return null;

  if (loading && results.length === 0 && !comparison && !statistics && !outcome) {
    return (
      <BacktestDetailSection title="Performance" className={className}>
        <p className="text-xs text-muted-foreground">Loading…</p>
      </BacktestDetailSection>
    );
  }

  const finalBalance =
    comparison && comparison.startCash > 0
      ? comparison.startCash * (1 + comparison.strategyReturn)
      : null;
  const totalReturnTone = comparison ? returnTone(comparison.strategyReturn) : "neutral";
  const trades = outcome ? tradeCountFromFills(outcome.fills) : null;
  const decisions = outcome?.decisions ?? null;
  const decisionRates = decisionTimestampsMs?.length
    ? decisionsPerMonthStats(decisionTimestampsMs)
    : null;

  const maxDd = kpiValue(results, "max-drawdown");
  const maxDdMeta = kpiMeta(results, "max-drawdown");
  const cagr = kpiValue(results, "cagr");
  const cagrMeta = kpiMeta(results, "cagr");
  const sharpe = kpiValue(results, "sharpe");
  const sharpeMeta = kpiMeta(results, "sharpe");
  const sortino = kpiValue(results, "sortino");
  const sortinoMeta = kpiMeta(results, "sortino");
  const vsBh = kpiValue(results, "active-return");
  const vsBhMeta = kpiMeta(results, "active-return");

  return (
    <BacktestDetailSection title="Performance" className={cn(className)}>
      <div className="space-y-3">
        <BacktestMetricGroup
          title="Total range"
          hint="Whole backtest window. Totals depend on run length; compare runs using p.a. and p.m. below."
        >
          {statistics && (
            <BacktestMetricRow label="Run length" value={formatDurationMonths(statistics.durationMonths)} />
          )}
          {finalBalance != null && (
            <BacktestMetricRow
              label="Final balance"
              value={`${finalBalance.toLocaleString(undefined, { maximumFractionDigits: 0 })} USDC`}
              tone={totalReturnTone}
            />
          )}
          {comparison && (
            <>
              <BacktestMetricRow
                label="Total return (window)"
                value={formatStrategyKpiValue("percent", comparison.strategyReturn)}
                tone={totalReturnTone}
              />
              <BacktestMetricRow
                label="Buy & hold (window)"
                value={formatStrategyKpiValue("percent", comparison.buyHoldReturn)}
                tone={returnTone(comparison.buyHoldReturn)}
              />
            </>
          )}
          {vsBh != null && vsBhMeta && (
            <BacktestMetricRow
              label={vsBhMeta.label}
              title={vsBhMeta.title}
              value={formatStrategyKpiValue(vsBhMeta.format, vsBh)}
              tone={strategyKpiTone("active-return", vsBh)}
            />
          )}
          {maxDd != null && maxDdMeta && (
            <BacktestMetricRow
              label={maxDdMeta.label}
              title={maxDdMeta.title}
              value={formatStrategyKpiValue(maxDdMeta.format, maxDd)}
              tone={strategyKpiTone("max-drawdown", maxDd)}
            />
          )}
          {trades != null && (
            <BacktestMetricRow label="Trades" value={trades.toLocaleString()} />
          )}
          {decisions != null && (
            <BacktestMetricRow label="Decisions" value={decisions.toLocaleString()} />
          )}
        </BacktestMetricGroup>

        <BacktestMetricGroup
          title="Per annum (p.a.)"
          hint="CAGR annualizes the full window. Year μ/med are mean and median of calendar years inside the run."
        >
          {cagr != null && cagrMeta && (
            <BacktestMetricRow
              label={cagrMeta.label}
              title={cagrMeta.title}
              value={formatStrategyKpiValue(cagrMeta.format, cagr)}
              tone={strategyKpiTone("cagr", cagr)}
            />
          )}
          {sharpe != null && sharpeMeta && (
            <BacktestMetricRow
              label={sharpeMeta.label}
              title={sharpeMeta.title}
              value={formatStrategyKpiValue(sharpeMeta.format, sharpe)}
              tone={strategyKpiTone("sharpe", sharpe)}
            />
          )}
          {sortino != null && sortinoMeta && (
            <BacktestMetricRow
              label={sortinoMeta.label}
              title={sortinoMeta.title}
              value={formatStrategyKpiValue(sortinoMeta.format, sortino)}
              tone={strategyKpiTone("sortino", sortino)}
            />
          )}
          {statistics && (
            <>
              <BacktestMetricRow
                label="Year return μ"
                value={formatStrategyKpiValue("percent", statistics.yearlyReturnAvg ?? NaN)}
                tone={
                  statistics.yearlyReturnAvg != null
                    ? returnTone(statistics.yearlyReturnAvg)
                    : "neutral"
                }
              />
              <BacktestMetricRow
                label="Year return med"
                value={formatStrategyKpiValue("percent", statistics.yearlyReturnMed ?? NaN)}
                tone={
                  statistics.yearlyReturnMed != null
                    ? returnTone(statistics.yearlyReturnMed)
                    : "neutral"
                }
              />
            </>
          )}
        </BacktestMetricGroup>

        {statistics && (
          <BacktestMetricGroup
            title="Per month (p.m.)"
            hint="μ / med across calendar months in this run."
          >
            <BacktestMetricRow
              label="Month return μ"
              value={formatStrategyKpiValue("percent", statistics.monthlyReturnAvg ?? NaN)}
              tone={
                statistics.monthlyReturnAvg != null
                  ? returnTone(statistics.monthlyReturnAvg * 12)
                  : "neutral"
              }
            />
            <BacktestMetricRow
              label="Month return med"
              value={formatStrategyKpiValue("percent", statistics.monthlyReturnMed ?? NaN)}
              tone={
                statistics.monthlyReturnMed != null
                  ? returnTone(statistics.monthlyReturnMed * 12)
                  : "neutral"
              }
            />
            <BacktestMetricRow
              label="Trades / mo μ"
              value={formatRate(statistics.tradesPerMonthAvg)}
            />
            <BacktestMetricRow
              label="Trades / mo med"
              value={formatRate(statistics.tradesPerMonthMed)}
            />
            {decisionRates && (
              <>
                <BacktestMetricRow
                  label="Decisions / mo μ"
                  value={formatRate(decisionRates.avg)}
                />
                <BacktestMetricRow
                  label="Decisions / mo med"
                  value={formatRate(decisionRates.med)}
                />
              </>
            )}
          </BacktestMetricGroup>
        )}

        {comparison && (
          <p className="text-xs text-muted-foreground">
            Benchmark: {comparison.startCash.toLocaleString()} USDC into the asset at the first bar (buy
            &amp; hold on the same window).
          </p>
        )}
      </div>
    </BacktestDetailSection>
  );
}
