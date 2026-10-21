import { ActivityView } from "@/components/activity-view";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { readActivity, readJournals, readLake } from "@/lib/read";

export const dynamic = "force-dynamic";

export default function OrdersPage() {
  const activity = readActivity();
  const lake = readLake();
  const journals = readJournals();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Orders</h1>
        <p className="text-sm text-muted-foreground">Decisions and fills, newest candle first.</p>
      </div>
      <Card className="max-w-xs">
        <CardHeader>
          <CardTitle>Decisions</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-2xl font-semibold tabular-nums">{lake.actions}</p>
          <p className="text-xs text-muted-foreground">{lake.fills} fills stored</p>
        </CardContent>
      </Card>
      {journals.length > 0 && (
        <div className="space-y-2 text-sm">
          <h2 className="font-medium">Fills</h2>
          <ul className="space-y-1 text-muted-foreground">
            {journals.map((row) => (
              <li key={row.orderLinkId}>
                {row.side} {row.baseQty} {row.venueSymbol} for {row.quoteQty} at {row.price}. Fee {row.fee || "—"} {row.feeCurrency}. Price input {row.priceSource}. Venue {row.venue}.
              </li>
            ))}
          </ul>
        </div>
      )}
      <ActivityView rows={activity} />
    </div>
  );
}
