import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { WikiMarkdown } from "@/components/wiki-markdown";
import { WikiMetaTable } from "@/components/wiki-meta";
import { WikiSeriesTable } from "@/components/wiki-series-table";
import type { StrategyKpiEntry } from "@/types/strategy-kpi";
import { formatStrategyKpiValue } from "@/lib/strategy-kpi-shared";
import type { IndicatorSeries, WikiEntry } from "@/types/wiki";

export function WikiIndicatorDetail({
  entry,
  series,
}: {
  entry: WikiEntry;
  series: IndicatorSeries[];
}) {
  return (
    <article className="space-y-4">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">{entry.meta.title}</h1>
        <p className="mt-1 text-sm text-muted-foreground">Indicator formula · {entry.meta.slug}</p>
      </div>
      <WikiMetaTable meta={entry.meta} />
      <WikiSeriesTable series={series} />
      <Card className="bg-card shadow-md ring-1 ring-foreground/10">
        <CardHeader className="border-b border-border/60 pb-4">
          <CardTitle className="text-sm font-semibold">Documentation</CardTitle>
        </CardHeader>
        <CardContent className="pt-4">
          <WikiMarkdown content={entry.content} />
        </CardContent>
      </Card>
    </article>
  );
}

export function WikiPlaybookDetail({ entry, kicker = "Playbook" }: { entry: WikiEntry; kicker?: string }) {
  return (
    <article className="space-y-4">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">{entry.meta.title}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{kicker} · {entry.meta.slug}</p>
      </div>
      <WikiMetaTable meta={entry.meta} />
      <Card className="bg-card shadow-md ring-1 ring-foreground/10">
        <CardHeader className="border-b border-border/60 pb-4">
          <CardTitle className="text-sm font-semibold">Documentation</CardTitle>
        </CardHeader>
        <CardContent className="pt-4">
          <WikiMarkdown content={entry.content} />
        </CardContent>
      </Card>
    </article>
  );
}

export function WikiStrategyKpiDetail({ entry }: { entry: StrategyKpiEntry }) {
  const { meta, content } = entry;
  return (
    <article className="space-y-4">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">{meta.title}</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Strategy performance indicator · <span className="font-mono">{meta.slug}</span>
        </p>
      </div>
      <Card className="bg-card shadow-md ring-1 ring-foreground/10">
        <CardHeader className="border-b border-border/60 pb-4">
          <CardTitle className="text-sm font-semibold">Metadata</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 pt-4 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="secondary">{meta.kind}</Badge>
            {meta.unit ? <Badge variant="outline">{meta.unit}</Badge> : null}
          </div>
          <dl className="grid gap-2 sm:grid-cols-[minmax(7rem,auto)_1fr]">
            <div className="contents">
              <dt className="text-muted-foreground">Label</dt>
              <dd>{meta.label}</dd>
            </div>
            <div className="contents">
              <dt className="text-muted-foreground">Display format</dt>
              <dd className="font-mono text-xs">{meta.format}</dd>
            </div>
            <div className="contents">
              <dt className="text-muted-foreground">Example</dt>
              <dd className="tabular-nums">{formatStrategyKpiValue(meta.format, 0.123)}</dd>
            </div>
          </dl>
        </CardContent>
      </Card>
      <Card className="bg-card shadow-md ring-1 ring-foreground/10">
        <CardHeader className="border-b border-border/60 pb-4">
          <CardTitle className="text-sm font-semibold">Documentation</CardTitle>
        </CardHeader>
        <CardContent className="pt-4">
          <WikiMarkdown content={content} />
        </CardContent>
      </Card>
    </article>
  );
}
