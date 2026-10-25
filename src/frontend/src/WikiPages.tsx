import { Badge } from "@/components/ui/badge";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { WikiIndexCard } from "@/components/wiki-index-card";
import {
  WikiIndicatorDetail,
  WikiPlaybookDetail,
  WikiStrategyKpiDetail,
} from "@/components/wiki-entry-detail";
import { useNavigate, useParams } from "react-router-dom";
import {
  indicatorSeries,
  wikiIndicators,
  wikiPlaybooks,
  wikiStrategyKpis,
} from "./wikiCatalog";

const SECTION_GRID = "grid grid-cols-1 items-start gap-8 lg:grid-cols-2";
const CARD_LIST = "grid grid-cols-1 gap-3 [&>*]:min-w-0";

type WikiKind = "indicators" | "playbooks" | "strategy-kpi";

export function WikiHome() {
  const params = useParams();
  const navigate = useNavigate();
  const splat = params["*"] ?? "";
  const [kind, slug] = splat.split("/") as [WikiKind | "", string | undefined];
  const open = Boolean(kind && slug);
  const indicators = wikiIndicators();
  const playbooks = wikiPlaybooks();
  const marketActions = playbooks.filter((entry) => entry.meta.kind !== "method");
  const positionSizing = playbooks.filter((entry) => entry.meta.kind === "method");
  const strategyKpis = wikiStrategyKpis();
  const title =
    kind === "indicators"
      ? indicators.find((entry) => entry.meta.slug === slug)?.meta.title
      : kind === "playbooks"
        ? playbooks.find((entry) => entry.meta.slug === slug)?.meta.title
        : strategyKpis.find((entry) => entry.meta.slug === slug)?.meta.title;

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <header className="mb-4 shrink-0">
        <h1 className="text-lg font-semibold tracking-tight">Wiki</h1>
        <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
          Pick an entry from the grid; its documentation opens in the panel on the right.
        </p>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <WikiNav
          indicators={indicators}
          marketActions={marketActions}
          positionSizing={positionSizing}
          strategyKpis={strategyKpis}
        />
      </div>
      <Sheet open={open} onOpenChange={(next) => { if (!next) navigate("/wiki"); }}>
        <SheetContent className="flex w-full flex-col gap-0 overflow-hidden p-0 data-[side=right]:w-full data-[side=right]:sm:max-w-xl">
          <SheetHeader className="border-b px-4 py-3 pr-12">
            <SheetTitle>{title || "Documentation"}</SheetTitle>
          </SheetHeader>
          <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
            <WikiDetail
              kind={kind}
              slug={slug ?? ""}
              indicators={indicators}
              playbooks={playbooks}
              strategyKpis={strategyKpis}
            />
          </div>
        </SheetContent>
      </Sheet>
    </div>
  );
}

function WikiDetail({
  kind,
  slug,
  indicators,
  playbooks,
  strategyKpis,
}: {
  kind: string;
  slug: string;
  indicators: ReturnType<typeof wikiIndicators>;
  playbooks: ReturnType<typeof wikiPlaybooks>;
  strategyKpis: ReturnType<typeof wikiStrategyKpis>;
}) {
  if (kind === "indicators") {
    const entry = indicators.find((item) => item.meta.slug === slug);
    if (!entry) return <Missing />;
    return <WikiIndicatorDetail entry={entry} series={indicatorSeries(entry)} />;
  }
  if (kind === "playbooks") {
    const entry = playbooks.find((item) => item.meta.slug === slug);
    if (!entry) return <Missing />;
    const kicker = entry.meta.kind === "method" ? "Position sizing" : "Playbook";
    return <WikiPlaybookDetail entry={entry} kicker={kicker} />;
  }
  if (kind === "strategy-kpi") {
    const entry = strategyKpis.find((item) => item.meta.slug === slug);
    if (!entry) return <Missing />;
    return <WikiStrategyKpiDetail entry={entry} />;
  }
  return null;
}

function WikiNav({
  indicators,
  marketActions,
  positionSizing,
  strategyKpis,
}: {
  indicators: ReturnType<typeof wikiIndicators>;
  marketActions: ReturnType<typeof wikiPlaybooks>;
  positionSizing: ReturnType<typeof wikiPlaybooks>;
  strategyKpis: ReturnType<typeof wikiStrategyKpis>;
}) {
  return (
    <div className={SECTION_GRID}>
      <section className="min-w-0 space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">Technical indicators</h2>
        <div className={CARD_LIST}>
          {indicators.map((entry) => (
            <WikiIndexCard
              key={entry.meta.slug}
              href={`/wiki/indicators/${entry.meta.slug}`}
              title={entry.meta.title}
              slugLine={entry.meta.slug}
              metaLine={`${indicatorSeries(entry).length} series`}
              badges={
                entry.meta.unit ? (
                  <Badge variant="outline" className="text-[10px]">
                    {entry.meta.unit}
                  </Badge>
                ) : undefined
              }
            />
          ))}
        </div>
      </section>
      <section className="min-w-0 space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">Market Action</h2>
        <div className={CARD_LIST}>
          {marketActions.map((entry) => (
            <WikiIndexCard
              key={entry.meta.slug}
              href={`/wiki/playbooks/${entry.meta.slug}`}
              title={entry.meta.title}
              slugLine={entry.meta.slug}
              metaLine={
                [entry.meta.symbols?.join(", "), entry.meta.venue ? `orders on ${entry.meta.venue}` : ""]
                  .filter(Boolean)
                  .join(" · ") || undefined
              }
              badges={
                entry.meta.status ? (
                  <Badge variant="secondary" className="text-[10px]">
                    {entry.meta.status}
                  </Badge>
                ) : undefined
              }
            />
          ))}
        </div>
      </section>
      <section className="min-w-0 space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">Position Sizing</h2>
        <div className={CARD_LIST}>
          {positionSizing.map((entry) => (
            <WikiIndexCard
              key={entry.meta.slug}
              href={`/wiki/playbooks/${entry.meta.slug}`}
              title={entry.meta.title}
              slugLine={entry.meta.slug}
              metaLine="Equity risked to the stop"
              badges={
                entry.meta.status ? (
                  <Badge variant="secondary" className="text-[10px]">
                    {entry.meta.status}
                  </Badge>
                ) : undefined
              }
            />
          ))}
        </div>
      </section>
      <section className="min-w-0 space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">Strategy performance indicators</h2>
        <div className={CARD_LIST}>
          {strategyKpis.map((entry) => (
            <WikiIndexCard
              key={entry.meta.slug}
              href={`/wiki/strategy-kpi/${entry.meta.slug}`}
              title={entry.meta.title}
              slugLine={entry.meta.slug}
              metaLine={`Format: ${entry.meta.format}`}
              badges={
                <Badge variant="outline" className="text-[10px]">
                  {entry.meta.label}
                </Badge>
              }
            />
          ))}
        </div>
      </section>
    </div>
  );
}

function Missing() {
  return <p className="text-sm text-muted-foreground">This entry is not in the wiki.</p>;
}
