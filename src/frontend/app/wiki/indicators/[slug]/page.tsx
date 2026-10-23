export const dynamic = "force-dynamic";

import { notFound } from "next/navigation";
import { WikiIndicatorDetail } from "@/components/wiki-entry-detail";
import { getIndicator, listIndicatorSeries, listIndicators } from "@/lib/wiki";

type Props = {
  params: Promise<{ slug: string }>;
};

export function generateStaticParams() {
  return listIndicators().map((entry) => ({ slug: entry.meta.slug }));
}

export default async function IndicatorWikiPage({ params }: Props) {
  const { slug } = await params;
  const entry = getIndicator(slug);
  if (!entry) {
    notFound();
  }
  return <WikiIndicatorDetail entry={entry} series={listIndicatorSeries(slug)} />;
}
