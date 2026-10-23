import { NextResponse } from "next/server";
import { readPlaybookTitleCatalog } from "@/lib/playbook-catalog";
import { readActivity, readJournals } from "@/lib/read";

export const dynamic = "force-dynamic";

export async function GET() {
  return NextResponse.json({
    rows: readActivity(),
    journals: readJournals(),
    playbookTitles: readPlaybookTitleCatalog(),
  });
}
