
import { BacktestDetailSection } from "@/components/backtest-detail-section";
import { Badge } from "@/components/ui/badge";
import type { BacktestRunStatistics, OutcomeSide } from "@/lib/backtest-period-stats";
import type { BacktestOutcome } from "@/lib/backtest-shared";
import type { BacktestKpiSetState } from "@/hooks/use-backtest-kpi-set";
import { returnTone, strategyKpiTone, toneTextClassName, type KpiTone } from "@/lib/strategy-kpi-tone";
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

type PeriodCell = { text: string; tone: KpiTone; title?: string };

const dash: PeriodCell = { text: "—", tone: "neutral" };

const periodHeaders: { label: string; title: string }[] = [
  { label: "Total", title: "Whole backtest window." },
  { label: "Month avg.", title: "Mean of calendar months in this run." },
  { label: "Month med.", title: "Median of calendar months in this run." },
  { label: "Year avg.", title: "Mean of calendar years inside the run." },
  { label: "Year med.", title: "Median of calendar years inside the run." },
];

const periodGrid =
  "grid grid-cols-[minmax(4.75rem,1.15fr)_repeat(5,minmax(0,1fr))] items-baseline gap-x-2";

function filled(cell: PeriodCell): boolean {
  return cell.text !== "—";
}

function percentCell(
  value: number | null | undefined,
  toneOf: (value: number) => KpiTone,
): PeriodCell {
  if (value == null || !Number.isFinite(value)) return dash;
  return { text: formatStrategyKpiValue("percent", value), tone: toneOf(value) };
}

function rateCell(value: number | null | undefined): PeriodCell {
  const text = formatRate(value);
  return text === "—" ? dash : { text, tone: "neutral" };
}

function PeriodValue({ cell }: { cell: PeriodCell }) {
  const empty = !filled(cell);
  const color = empty
    ? "font-normal text-muted-foreground"
    : cell.tone === "neutral"
      ? "text-foreground"
      : toneTextClassName(cell.tone);
  return (
    <span className={cn("text-right text-xs font-medium tabular-nums", color)} title={cell.title}>
      {cell.text}
    </span>
  );
}

function KpiPeriodRow({
  label,
  title,
  cells,
}: {
  label: string;
  title?: string;
  cells: PeriodCell[];
}) {
  return (
    <div className={cn(periodGrid, "border-b border-border/40 py-1.5 last:border-b-0")}>
      <span className="text-xs text-muted-foreground" title={title}>
        {label}
      </span>
      {cells.map((cell, index) => (
        <PeriodValue key={index} cell={cell} />
      ))}
    </div>
  );
}

function singleTotal(cell: PeriodCell): PeriodCell[] {
  return [cell, dash, dash, dash, dash];
}

function HeaderTag({
  label,
  value,
  valueClassName,
}: {
  label: string;
  value: string;
  valueClassName?: string;
}) {
  return (
    <Badge variant="secondary" className="font-normal tabular-nums">
      <span className="text-muted-foreground">{label}</span>
      <span className={cn("font-medium text-foreground", valueClassName)}>{value}</span>
    </Badge>
  );
}

const ladderKeys = ["average", "p50", "p75", "p90", "p95"] as const;

const ladderHeaders = ["Average", "Median", "25% above", "10% above", "5% above"];

function formatQuote(value: number | undefined): string {
  if (value == null || !Number.isFinite(value)) return "—";
  return value.toLocaleString(undefined, { maximumFractionDigits: 2 });
}

function LadderValue({
  side,
  tone,
}: {
  side: OutcomeSide | null | undefined;
  tone: KpiTone;
}) {
  return (
    <>
      {ladderKeys.map((key) => {
        const quote = side?.absolute?.[key];
        const pct = side?.percentage?.[key];
        const color = toneTextClassName(tone);
        return (
          <span key={key} className="text-right text-xs font-medium tabular-nums leading-tight">
            <span className={quote == null ? "font-normal text-muted-foreground" : color}>
              {formatQuote(quote)}
            </span>
            <span className={cn("block", pct == null ? "font-normal text-muted-foreground" : color)}>
              {pct == null ? "—" : formatStrategyKpiValue("percent", pct)}
            </span>
          </span>
        );
      })}
    </>
  );
}

function OutcomeLadder({
  label,
  side,
  tone,
}: {
  label: string;
  side: OutcomeSide | null | undefined;
  tone: KpiTone;
}) {
  if (!side?.absolute && !side?.percentage) return null;
  return (
    <div className={cn(periodGrid, "border-b border-border/40 py-1.5 last:border-b-0")}>
      <span className="text-xs text-muted-foreground">{label}</span>
      <LadderValue side={side} tone={tone} />
    </div>
  );
}

export function BacktestPerformancePanel({
  kpi,
  comparison,
  statistics,
  outcome,
  loading,
  className,
  title = "Performance",
}: {
  kpi: BacktestKpiSetState;
  comparison: StrategyKpiComparison | undefined;
  statistics: BacktestRunStatistics | undefined;
  outcome: BacktestOutcome | null;
  decisionTimestampsMs?: number[];
  loading?: boolean;
  className?: string;
  title?: string;
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
      <BacktestDetailSection title={title} className={className}>
        <p className="text-xs text-muted-foreground">Loading…</p>
      </BacktestDetailSection>
    );
  }

  const finalBalance =
    comparison && comparison.startCash > 0
      ? comparison.startCash * (1 + comparison.strategyReturn)
      : null;
  const totalReturnTone = comparison ? returnTone(comparison.strategyReturn) : "neutral";

  const rows: { label: string; title?: string; cells: PeriodCell[] }[] = [];

  const returnCells: PeriodCell[] = [
    comparison
      ? {
          text: formatStrategyKpiValue("percent", comparison.strategyReturn),
          tone: totalReturnTone,
        }
      : dash,
    percentCell(statistics?.monthlyReturnAvg, (value) => returnTone(value * 12)),
    percentCell(statistics?.monthlyReturnMed, (value) => returnTone(value * 12)),
    percentCell(statistics?.yearlyReturnAvg, returnTone),
    percentCell(statistics?.yearlyReturnMed, returnTone),
  ];
  if (returnCells.some(filled)) {
    rows.push({ label: "Return", cells: returnCells });
  }

  if (comparison) {
    rows.push({
      label: "Buy & hold",
      cells: singleTotal({
        text: formatStrategyKpiValue("percent", comparison.buyHoldReturn),
        tone: returnTone(comparison.buyHoldReturn),
      }),
    });
  }

  const vsBh = kpiValue(results, "active-return");
  const vsBhMeta = kpiMeta(results, "active-return");
  if (vsBh != null && vsBhMeta) {
    rows.push({
      label: vsBhMeta.label,
      title: vsBhMeta.title,
      cells: singleTotal({
        text: formatStrategyKpiValue(vsBhMeta.format, vsBh),
        tone: strategyKpiTone("active-return", vsBh),
        title: vsBhMeta.title,
      }),
    });
  }

  for (const slug of ["cagr", "sharpe", "sortino", "max-drawdown"] as const) {
    const value = kpiValue(results, slug);
    const meta = kpiMeta(results, slug);
    if (value == null || !meta) continue;
    rows.push({
      label: meta.label,
      title: meta.title,
      cells: singleTotal({
        text: formatStrategyKpiValue(meta.format, value),
        tone: strategyKpiTone(slug, value),
        title: meta.title,
      }),
    });
  }

  const winCells: PeriodCell[] = [
    percentCell(statistics?.winRate, (value) => strategyKpiTone("win-rate", value)),
    percentCell(statistics?.monthlyWinRateAvg, (value) => strategyKpiTone("win-rate", value)),
    percentCell(statistics?.monthlyWinRateMed, (value) => strategyKpiTone("win-rate", value)),
    percentCell(statistics?.yearlyWinRateAvg, (value) => strategyKpiTone("win-rate", value)),
    percentCell(statistics?.yearlyWinRateMed, (value) => strategyKpiTone("win-rate", value)),
  ];
  if (winCells.some(filled)) {
    rows.push({
      label: "Win rate",
      title: "Winning sells divided by closed trades. Month and year cells are the mean and median of calendar-bucket rates.",
      cells: winCells,
    });
  }

  const tradeAvg = rateCell(statistics?.tradesPerMonthAvg);
  const tradeMed = rateCell(statistics?.tradesPerMonthMed);
  if (filled(tradeAvg) || filled(tradeMed)) {
    rows.push({
      label: "Trades",
      cells: [dash, tradeAvg, tradeMed, dash, dash],
    });
  }

  const showRunLength = (statistics?.durationMonths ?? 0) > 0;
  const showLadder = Boolean(statistics?.gain || statistics?.loss);
  const showFinalBalance = finalBalance != null;

  const balanceTone = totalReturnTone === "neutral" ? undefined : toneTextClassName(totalReturnTone);

  return (
    <BacktestDetailSection
      title={title}
      className={cn(className)}
      titleExtra={
        showRunLength || showFinalBalance ? (
          <>
            {showRunLength && statistics && (
              <HeaderTag label="Run length" value={formatDurationMonths(statistics.durationMonths)} />
            )}
            {showFinalBalance && (
              <HeaderTag
                label="Final balance"
                value={`${finalBalance.toLocaleString(undefined, { maximumFractionDigits: 0 })} USDC`}
                valueClassName={balanceTone}
              />
            )}
          </>
        ) : null
      }
    >
      <div className="space-y-4">
        {rows.length > 0 && (
          <div>
            <div className={cn(periodGrid, "border-b border-border/40 pb-1")}>
              <span />
              {periodHeaders.map((header) => (
                <span
                  key={header.label}
                  className="text-right text-[0.65rem] leading-tight text-muted-foreground"
                  title={header.title}
                >
                  {header.label}
                </span>
              ))}
            </div>
            {rows.map((row) => (
              <KpiPeriodRow key={row.label} label={row.label} title={row.title} cells={row.cells} />
            ))}
          </div>
        )}
        {showLadder && (
          <div>
            <div className={cn(periodGrid, "border-b border-border/40 pb-1")}>
              <span />
              {ladderHeaders.map((header) => (
                <span
                  key={header}
                  className="text-right text-[0.65rem] leading-tight text-muted-foreground"
                >
                  {header}
                </span>
              ))}
            </div>
            <OutcomeLadder label="Gain" side={statistics?.gain} tone="good" />
            <OutcomeLadder label="Loss" side={statistics?.loss} tone="bad" />
          </div>
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
