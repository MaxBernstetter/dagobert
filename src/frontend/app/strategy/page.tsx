import { StrategyActivityPanel } from "@/components/strategy-activity-panel";
import { readLive } from "@/lib/read";

export const dynamic = "force-dynamic";

export default function StrategyPage() {
  const live = readLive();

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <StrategyActivityPanel live={live} className="min-h-0 flex-1" />
    </div>
  );
}
