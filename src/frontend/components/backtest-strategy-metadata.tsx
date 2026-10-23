import Link from "next/link";
import { BacktestDetailSection, BacktestMetricTile } from "@/components/backtest-detail-section";
import { formatBacktestRange } from "@/lib/backtest-shared";

function playbookWikiSlug(playbook: string): string {
  return playbook.replace(/\.ya?ml$/i, "");
}

export function BacktestStrategyMetadata({
  strategyTitle,
  playbook,
  symbol,
  fromMs,
  toMs,
}: {
  strategyTitle: string;
  playbook: string;
  symbol: string;
  fromMs: number;
  toMs: number;
}) {
  const wikiSlug = playbookWikiSlug(playbook);

  return (
    <BacktestDetailSection title="Strategy Metadata">
      <div className="flex flex-wrap gap-2">
        <BacktestMetricTile
          label="Strategy"
          className="min-w-[8rem] px-3 py-2"
          value={
            <Link
              href={`/wiki/playbooks/${wikiSlug}`}
              className="underline-offset-2 hover:underline"
            >
              {strategyTitle}
            </Link>
          }
          valueClassName="text-sm font-medium"
        />
        <BacktestMetricTile
          label="Asset"
          className="min-w-[6rem] px-3 py-2"
          value={symbol}
          valueClassName="font-mono text-sm"
        />
        <BacktestMetricTile
          label="From – to"
          className="min-w-[10rem] px-3 py-2"
          value={formatBacktestRange(fromMs, toMs)}
          valueClassName="text-sm font-medium whitespace-nowrap"
        />
      </div>
    </BacktestDetailSection>
  );
}
