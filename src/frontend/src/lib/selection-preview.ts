/** Closed-control preview: up to `maxVisible` values, then "... n more". */
export function formatValuePreview(
  values: string[],
  maxVisible = 3,
  empty = "None selected",
): string {
  const cleaned = values.map((v) => v.trim()).filter(Boolean);
  if (cleaned.length === 0) return empty;
  const shown = cleaned.slice(0, maxVisible);
  const head = shown.join(" · ");
  if (cleaned.length <= maxVisible) return head;
  return `${head} ... ${cleaned.length - maxVisible} more`;
}
