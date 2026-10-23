import fs from "fs";
import path from "path";
import matter from "gray-matter";

export type WikiMeta = {
  title: string;
  slug: string;
  kind: string;
  unit?: string;
  status?: string;
  algorithm?: string;
  playbook_file?: string;
  definition?: Record<string, unknown>;
  inputs?: string[];
  symbols?: string[];
  venue?: string;
  price_source?: string;
  series?: IndicatorSeries[];
};

export type WikiEntry = {
  meta: WikiMeta;
  content: string;
};

export type IndicatorSeries = {
  key: string;
  definition: Record<string, unknown>;
};

const indicatorSkipDirs = new Set(["types"]);

function repoRoot(): string {
  if (process.env.REPO_ROOT) {
    return process.env.REPO_ROOT;
  }
  return path.resolve(process.cwd(), "..", "..");
}

function hasIndicatorDocs(dir: string): boolean {
  if (!fs.existsSync(dir)) {
    return false;
  }
  return fs.readdirSync(dir, { withFileTypes: true }).some(
    (entry) =>
      entry.isDirectory() &&
      !indicatorSkipDirs.has(entry.name) &&
      fs.existsSync(path.join(dir, entry.name, "README.md")),
  );
}

function resolveCatalogDir(): string {
  const candidates = [
    process.env.WIKI_CATALOG_DIR,
    "/wiki/calculation-technical-indicator/internal/indicators",
    "/wiki/calculation-technical-indicator/indicators",
    path.join(repoRoot(), "src", "calculation-technical-indicator", "internal", "indicators"),
    path.join(repoRoot(), "src", "calculation-technical-indicator", "indicators"),
  ].filter((dir): dir is string => Boolean(dir));

  for (const dir of candidates) {
    if (hasIndicatorDocs(dir)) {
      return dir;
    }
  }
  return path.join(repoRoot(), "src", "calculation-technical-indicator", "internal", "indicators");
}

function playbooksDir(): string {
  if (process.env.WIKI_PLAYBOOKS_DIR) {
    return process.env.WIKI_PLAYBOOKS_DIR;
  }
  const candidates = [
    "/wiki/playbooks",
    path.join(repoRoot(), "src", "calculation-signal", "playbooks"),
  ];
  for (const dir of candidates) {
    if (fs.existsSync(dir)) {
      return dir;
    }
  }
  return path.join(repoRoot(), "src", "calculation-signal", "playbooks");
}

function readMarkdownEntry(root: string, slug: string, filename: string): WikiEntry | null {
  const file = path.join(root, slug, filename);
  if (!fs.existsSync(file)) {
    return null;
  }
  const raw = fs.readFileSync(file, "utf8");
  const { data, content } = matter(raw);
  const meta = data as WikiMeta;
  if (!meta.slug) {
    meta.slug = slug;
  }
  return { meta, content: content.trim() };
}

function listMarkdownEntries(root: string, filename: string, skip?: Set<string>): WikiEntry[] {
  if (!fs.existsSync(root)) {
    return [];
  }
  return fs
    .readdirSync(root, { withFileTypes: true })
    .filter(
      (entry) =>
        entry.isDirectory() &&
        !(skip?.has(entry.name) ?? false) &&
        fs.existsSync(path.join(root, entry.name, filename)),
    )
    .map((entry) => readMarkdownEntry(root, entry.name, filename))
    .filter((entry): entry is WikiEntry => entry !== null)
    .sort((a, b) => a.meta.title.localeCompare(b.meta.title));
}

/** Deployed series for one formula (from README frontmatter; kept in sync by Go tests). */
export function listIndicatorSeries(formulaSlug: string): IndicatorSeries[] {
  const entry = getIndicator(formulaSlug);
  if (!entry?.meta.series?.length) {
    return [];
  }
  return [...entry.meta.series].sort((a, b) => a.key.localeCompare(b.key));
}

export function listIndicators(): WikiEntry[] {
  return listMarkdownEntries(resolveCatalogDir(), "README.md", indicatorSkipDirs);
}

export function listPlaybooks(): WikiEntry[] {
  return listMarkdownEntries(playbooksDir(), "DESCRIPTION.md");
}

export function getIndicator(slug: string): WikiEntry | null {
  return readMarkdownEntry(resolveCatalogDir(), slug, "README.md");
}

export function getPlaybook(slug: string): WikiEntry | null {
  return readMarkdownEntry(playbooksDir(), slug, "DESCRIPTION.md");
}
