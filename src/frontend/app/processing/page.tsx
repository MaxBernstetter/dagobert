import { BackfillForm } from "@/components/backfill-form";
import { LiveStatus } from "@/components/live-status";
import { readLive } from "@/lib/read";

export const dynamic = "force-dynamic";

export default function ProcessingPage() {
  const live = readLive();

  return (
    <div className="min-h-0 flex-1 space-y-8 overflow-y-auto">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Processing</h1>
        <p className="text-sm text-muted-foreground">
          Backfill candles or recalculate indicators for the positions you choose.
        </p>
      </div>
      <LiveStatus live={live} />
      <BackfillForm
        initial={{
          kind: "",
          from: 0,
          to: 0,
          symbol: "",
          running: false,
          error: "",
          finished: "",
        }}
      />
    </div>
  );
}
