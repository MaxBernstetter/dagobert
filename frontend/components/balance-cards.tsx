import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { BalanceRow } from "@/lib/read";

function formatTime(ms: number): string {
  if (!ms) return "—";
  return new Date(ms).toISOString().replace("T", " ").replace(/\.\d{3}Z$/, " UTC");
}

export function BalanceCards({ balances }: { balances: BalanceRow[] }) {
  if (balances.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">No balance snapshot yet.</p>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {balances.map((b) => (
        <Card key={b.currency} size="sm">
          <CardHeader>
            <CardTitle>{b.currency}</CardTitle>
            <CardDescription>Latest cash balance</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold tabular-nums">{b.cashBalance}</p>
            <p className="text-xs text-muted-foreground mt-2">
              Updated {formatTime(b.transactionTime)}
            </p>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
