// Wiki entry shapes. The SPA gets these from the Experiment API, not from the filesystem.

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
