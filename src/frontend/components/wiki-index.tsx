import { Badge } from "@/components/ui/badge";
import { WikiIndexCard } from "@/components/wiki-index-card";
import { listStrategyKpis } from "@/lib/strategy-kpi-catalog";
import { listIndicatorSeries, listIndicators, listPlaybooks } from "@/lib/wiki";

const ITEM_GRID =
  "grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 [&>*]:min-w-0";

export function WikiIndex() {
  const indicators = listIndicators();
  const playbooks = listPlaybooks();
  const strategyKpis = listStrategyKpis();

  return (
    <nav className="space-y-8" aria-label="Wiki catalog">
      <section className="space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">Technical indicators</h2>
        <div className={ITEM_GRID}>
          {indicators.map((entry) => {
            const series = listIndicatorSeries(entry.meta.slug);
            return (
              <WikiIndexCard
                key={entry.meta.slug}
                href={`/wiki/indicators/${entry.meta.slug}`}
                title={entry.meta.title}
                slugLine={entry.meta.slug}
                metaLine={`${series.length} series`}
                badges={
                  entry.meta.unit ? <Badge variant="outline" className="text-[10px]">{entry.meta.unit}</Badge> : undefined
                }
              />
            );
          })}
        </div>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">Trading strategies (playbooks)</h2>
        <div className={ITEM_GRID}>
          {playbooks.map((entry) => {
            const metaParts = [
              entry.meta.symbols?.join(", "),
              entry.meta.venue ? `orders on ${entry.meta.venue}` : "",
            ].filter(Boolean);
            return (
              <WikiIndexCard
                key={entry.meta.slug}
                href={`/wiki/playbooks/${entry.meta.slug}`}
                title={entry.meta.title}
                slugLine={entry.meta.slug}
                metaLine={metaParts.join(" · ") || undefined}
                badges={
                  entry.meta.status ? (
                    <Badge variant="secondary" className="text-[10px]">{entry.meta.status}</Badge>
                  ) : undefined
                }
              />
            );
          })}
        </div>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">Strategy performance indicators</h2>
        <p className="text-sm text-muted-foreground">
          Metrics on the Strategy page; computed from fills and bar-close marks.
        </p>
        <div className={ITEM_GRID}>
          {strategyKpis.map((entry) => (
            <WikiIndexCard
              key={entry.meta.slug}
              href={`/wiki/strategy-kpi/${entry.meta.slug}`}
              title={entry.meta.title}
              slugLine={entry.meta.slug}
              metaLine={`Format: ${entry.meta.format}`}
              badges={<Badge variant="outline" className="text-[10px]">{entry.meta.label}</Badge>}
            />
          ))}
        </div>
      </section>
    </nav>
  );
}
