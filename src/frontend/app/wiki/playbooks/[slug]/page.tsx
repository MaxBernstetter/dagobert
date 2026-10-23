export const dynamic = "force-dynamic";

import { notFound } from "next/navigation";
import { WikiPlaybookDetail } from "@/components/wiki-entry-detail";
import { getPlaybook, listPlaybooks } from "@/lib/wiki";

type Props = {
  params: Promise<{ slug: string }>;
};

export function generateStaticParams() {
  return listPlaybooks().map((entry) => ({ slug: entry.meta.slug }));
}

export default async function PlaybookWikiPage({ params }: Props) {
  const { slug } = await params;
  const entry = getPlaybook(slug);
  if (!entry) {
    notFound();
  }
  return <WikiPlaybookDetail entry={entry} />;
}
