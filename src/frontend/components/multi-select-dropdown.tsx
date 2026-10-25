"use client";

import { useMemo, useState } from "react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { formatValuePreview } from "@/lib/selection-preview";
import { cn } from "@/lib/utils";

const fieldInputClass =
  "w-full rounded-md border border-border bg-background px-2 py-1.5 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50";

export type MultiSelectOption = {
  value: string | number;
  label: string;
};

type Props = {
  label: string;
  options: MultiSelectOption[];
  selected: Array<string | number>;
  onChange: (selected: Array<string | number>) => void;
  /** Show a search box (useful for long lists). */
  searchable?: boolean;
  disabled?: boolean;
  emptyHint?: string;
  /** Defaults to form-field muted caption. */
  labelClassName?: string;
  /**
   * Closed trigger text: `count` → "N selected" (or "N selected {countUnit}");
   * `values` → up to 3 labels joined with " · ", then "... n more".
   */
  preview?: "count" | "values";
  /** When `preview="count"`, e.g. "pairs" → "3 selected pairs". */
  countUnit?: string;
  /** Mono + truncate — match Symbol / market controls. */
  triggerClassName?: string;
};

function valuesEqual(a: string | number, b: string | number): boolean {
  return a === b || String(a) === String(b);
}

export function MultiSelectDropdown({
  label,
  options,
  selected,
  onChange,
  searchable = false,
  disabled,
  emptyHint = "No options",
  labelClassName = "text-muted-foreground",
  preview = "count",
  countUnit,
  triggerClassName,
}: Props) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  const filtered = useMemo(() => {
    const q = query.trim().toUpperCase();
    if (!q) return options;
    return options.filter(
      (o) =>
        o.label.toUpperCase().includes(q) ||
        String(o.value).toUpperCase().includes(q),
    );
  }, [options, query]);

  const triggerLabel = useMemo(() => {
    if (preview === "count") {
      if (selected.length === 0) return "None selected";
      const unit = countUnit?.trim();
      if (!unit) return `${selected.length} selected`;
      return `${selected.length} selected ${unit}`;
    }
    const labels = selected.map((s) => {
      const match = options.find((o) => valuesEqual(o.value, s));
      return match?.label ?? String(s);
    });
    return formatValuePreview(labels);
  }, [preview, countUnit, selected, options]);

  return (
    <label className="grid gap-1">
      <span className={labelClassName}>{label}</span>
      <Popover
        open={open}
        onOpenChange={(next) => {
          setOpen(next);
          if (!next) setQuery("");
        }}
      >
        <PopoverTrigger
          disabled={disabled}
          className={cn(fieldInputClass, "truncate text-left", triggerClassName)}
        >
          {triggerLabel}
        </PopoverTrigger>
        <PopoverContent className="w-[var(--anchor-width)] p-2" align="start">
          {searchable && (
            <input
              autoFocus
              className={fieldInputClass}
              placeholder="Search…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          )}
          <div className={cn("max-h-64 overflow-auto", searchable && "mt-2")}>
            {filtered.length === 0 && (
              <p className="px-1 py-2 text-xs text-muted-foreground">{emptyHint}</p>
            )}
            {filtered.map((o) => {
              const checked = selected.some((s) => valuesEqual(s, o.value));
              return (
                <label
                  key={String(o.value)}
                  className={cn(
                    "flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-sm hover:bg-muted",
                    checked && "bg-muted",
                  )}
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => {
                      onChange(
                        checked
                          ? selected.filter((s) => !valuesEqual(s, o.value))
                          : [...selected, o.value],
                      );
                    }}
                  />
                  <span className="font-mono">{o.label}</span>
                </label>
              );
            })}
          </div>
        </PopoverContent>
      </Popover>
    </label>
  );
}
