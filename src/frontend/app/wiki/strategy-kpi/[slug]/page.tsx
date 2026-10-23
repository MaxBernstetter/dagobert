export const dynamic = "force-dynamic";

import { notFound } from "next/navigation";
import { WikiStrategyKpiDetail } from "@/components/wiki-entry-detail";
import { getStrategyKpi, listStrategyKpis } from "@/lib/strategy-kpi-catalog";

type Props = {
  params: Promise<{ slug: string }>;
};

export function generateStaticParams() {
  return listStrategyKpis().map((entry) => ({ slug: entry.meta.slug }));
}

export default async function StrategyKpiWikiPage({ params }: Props) {
  const { slug } = await params;
  const entry = getStrategyKpi(slug);
  if (!entry) {
    notFound();
  }
  return <WikiStrategyKpiDetail entry={entry} />;
}
