import { BacktestingPanel } from "@/components/backtesting-panel";
import type { PlaybookOption } from "@/lib/backtest-shared";
import { listPlaybooks } from "@/lib/wiki";

export const dynamic = "force-dynamic";

function deployedPlaybookOptions(): PlaybookOption[] {
  return listPlaybooks().map((entry) => ({
    id: entry.meta.slug,
    label: entry.meta.title?.trim() || entry.meta.slug,
  }));
}

export default function BacktestingPage() {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden p-4 lg:p-6">
      <BacktestingPanel className="min-h-0 flex-1" playbookOptions={deployedPlaybookOptions()} />
    </div>
  );
}
