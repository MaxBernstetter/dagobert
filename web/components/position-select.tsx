"use client";

import { useEffect, useMemo, useRef, useState } from "react";

const OPTIONS = [
  { id: "BTCUSDT", label: "BTC" },
  { id: "ETHUSDT", label: "ETH" },
  { id: "USDCUSDT", label: "USDC series" },
];

export function PositionSelect({
  value,
  onChange,
}: {
  value: string[];
  onChange: (next: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const box = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function close(event: MouseEvent) {
      if (!box.current?.contains(event.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, []);

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return OPTIONS.filter((option) => option.label.toLowerCase().includes(q) || option.id.toLowerCase().includes(q));
  }, [query]);

  const summary =
    value.length === 0
      ? "Select positions"
      : OPTIONS.filter((option) => value.includes(option.id))
          .map((option) => option.label)
          .join(", ");

  function toggle(id: string) {
    onChange(value.includes(id) ? value.filter((item) => item !== id) : [...value, id]);
  }

  return (
    <div ref={box} className="relative space-y-1 text-sm">
      <span className="text-muted-foreground">Positions</span>
      <button
        type="button"
        className="flex h-8 min-w-40 items-center rounded-md border bg-background px-2 text-left"
        onClick={() => setOpen((current) => !current)}
      >
        {summary}
      </button>
      {open && (
        <div className="absolute z-10 mt-1 w-56 rounded-md border bg-background p-2 shadow-md">
          <input
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search"
            className="mb-2 w-full rounded-md border bg-background px-2 py-1"
          />
          <ul className="space-y-1">
            {shown.map((option) => (
              <li key={option.id}>
                <label className="flex cursor-pointer items-center gap-2 rounded px-1 py-1 hover:bg-muted">
                  <input type="checkbox" checked={value.includes(option.id)} onChange={() => toggle(option.id)} />
                  <span>{option.label}</span>
                  <span className="text-xs text-muted-foreground">{option.id}</span>
                </label>
              </li>
            ))}
            {shown.length === 0 && <li className="px-1 text-xs text-muted-foreground">No matches</li>}
          </ul>
        </div>
      )}
    </div>
  );
}
