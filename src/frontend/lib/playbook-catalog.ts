import "server-only";

import { listPlaybooks } from "@/lib/wiki";

/** Playbook id (e.g. ma-cross.yaml) → display title from DESCRIPTION.md frontmatter. */
export function readPlaybookTitleCatalog(): Record<string, string> {
  const catalog: Record<string, string> = {};
  for (const entry of listPlaybooks()) {
    const title = entry.meta.title?.trim();
    if (!title) continue;
    const slug = entry.meta.slug?.trim() || "";
    if (slug) {
      catalog[slug] = title;
      catalog[`${slug}.yaml`] = title;
    }
    const file = entry.meta.playbook_file?.trim();
    if (file) {
      catalog[file] = title;
    }
  }
  return catalog;
}
