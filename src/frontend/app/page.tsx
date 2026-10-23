import { BalanceCards } from "@/components/balance-cards";
import { readBalances, readPositions } from "@/lib/read";

export const dynamic = "force-dynamic";

export default function HomePage() {
  const balances = readBalances();
  const positions = readPositions();

  return (
    <div className="min-h-0 flex-1 space-y-8 overflow-y-auto">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Portfolio</h1>
        <p className="text-sm text-muted-foreground">
          Cash is the latest snapshot. Positions are quantity and cost rebuilt from fills.
        </p>
      </div>
      <section className="space-y-3">
        <h2 className="text-lg font-medium">Positions</h2>
        {positions.length === 0 ? (
          <p className="text-sm text-muted-foreground">No open quantity.</p>
        ) : (
          <ul className="space-y-2 text-sm">
            {positions.map((position) => (
              <li key={position.venueSymbol}>
                {position.quantity} {position.base} ({position.venueSymbol}). Cost {position.costQuote} USDC. Average {position.avgPrice}.
              </li>
            ))}
          </ul>
        )}
      </section>
      <section className="space-y-3">
        <h2 className="text-lg font-medium">Cash</h2>
        <BalanceCards balances={balances} />
      </section>
    </div>
  );
}
