import { ActivityView } from "@/components/activity-view";
import { BalanceCards } from "@/components/balance-cards";
import { readActivity, readBalances } from "@/lib/read";

export const dynamic = "force-dynamic";

export default function HomePage() {
  const balances = readBalances();
  const activity = readActivity();

  return (
    <div className="space-y-10">
      <section className="space-y-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Overview</h1>
          <p className="text-sm text-muted-foreground">
            Balances and trading activity, newest candle first.
          </p>
        </div>
        <BalanceCards balances={balances} />
      </section>

      <section className="space-y-4">
        <h2 className="text-lg font-medium">Activity</h2>
        <ActivityView rows={activity} />
      </section>
    </div>
  );
}
