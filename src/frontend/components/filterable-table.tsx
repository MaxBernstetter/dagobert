"use client";

import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { cn } from "@/lib/utils";

export type FilterableColumnDef<T> = {
  id: string;
  header: React.ReactNode;
  cell: (row: T) => React.ReactNode;
  /** Value used for text / select filtering */
  filterValue?: (row: T) => string;
  filter?: "text" | "select";
  className?: string;
  /** When set (or `sortable` is true), column header is clickable to sort. */
  sortValue?: (row: T) => string | number | null | undefined;
  sortable?: boolean;
};

type SortDir = "asc" | "desc";

function columnSortable<T>(col: FilterableColumnDef<T>): boolean {
  return col.sortable === true || col.sortValue != null;
}

function compareSortValues(
  a: string | number | null | undefined,
  b: string | number | null | undefined,
  dir: SortDir,
): number {
  const mul = dir === "asc" ? 1 : -1;
  const aEmpty = a == null || a === "";
  const bEmpty = b == null || b === "";
  if (aEmpty && bEmpty) return 0;
  if (aEmpty) return 1;
  if (bEmpty) return -1;
  if (typeof a === "number" && typeof b === "number") {
    return mul * (a - b);
  }
  return mul * String(a).localeCompare(String(b), undefined, { numeric: true });
}

type FilterableTableProps<T> = {
  rows: T[];
  columns: FilterableColumnDef<T>[];
  rowKey: (row: T) => string;
  emptyMessage?: string;
  className?: string;
  onRowClick?: (row: T) => void;
  rowClassName?: (row: T, index: number) => string | undefined;
  isRowSelected?: (row: T) => boolean;
  footer?: React.ReactNode;
  /** When set, only this many filtered rows render per page. */
  pageSize?: number;
  /** When false, omit the inner bordered frame (use inside a Card). */
  framed?: boolean;
  /** When `message`, zero rows (no active filters) shows only emptyMessage — no table chrome. */
  emptyPresentation?: "table" | "message";
};

export function FilterableTable<T>({
  rows,
  columns,
  rowKey,
  emptyMessage = "No rows.",
  className,
  onRowClick,
  rowClassName,
  isRowSelected,
  footer,
  pageSize,
  framed = true,
  emptyPresentation = "table",
}: FilterableTableProps<T>) {
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [page, setPage] = useState(0);
  const [sortColumnId, setSortColumnId] = useState<string | null>(null);
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const selectOptions = useMemo(() => {
    const out: Record<string, string[]> = {};
    for (const col of columns) {
      if (col.filter !== "select") continue;
      const values = new Set<string>();
      for (const row of rows) {
        const v = (col.filterValue?.(row) ?? "").trim();
        if (v) values.add(v);
      }
      out[col.id] = [...values].sort();
    }
    return out;
  }, [columns, rows]);

  const filtered = useMemo(() => {
    return rows.filter((row) =>
      columns.every((col) => {
        const raw = filters[col.id]?.trim();
        if (!raw) return true;
        const value = (col.filterValue?.(row) ?? "").toLowerCase();
        if (col.filter === "select") {
          return value === raw.toLowerCase();
        }
        return value.includes(raw.toLowerCase());
      })
    );
  }, [rows, columns, filters]);

  const sorted = useMemo(() => {
    if (!sortColumnId) return filtered;
    const col = columns.find((c) => c.id === sortColumnId);
    if (!col || !columnSortable(col)) return filtered;
    const getVal = (row: T) =>
      col.sortValue?.(row) ?? col.filterValue?.(row) ?? null;
    return [...filtered].sort((a, b) => compareSortValues(getVal(a), getVal(b), sortDir));
  }, [filtered, sortColumnId, sortDir, columns]);

  useEffect(() => {
    setPage(0);
  }, [filters, rows.length, pageSize, sortColumnId, sortDir]);

  const pageCount =
    pageSize != null && pageSize > 0 ? Math.max(1, Math.ceil(sorted.length / pageSize)) : 1;
  const safePage = Math.min(page, pageCount - 1);
  const pageRows = useMemo(() => {
    if (pageSize == null || pageSize <= 0) return sorted;
    const start = safePage * pageSize;
    return sorted.slice(start, start + pageSize);
  }, [sorted, pageSize, safePage]);

  function setFilter(id: string, value: string) {
    setFilters((prev) => ({ ...prev, [id]: value }));
  }

  function toggleSort(col: FilterableColumnDef<T>) {
    if (!columnSortable(col)) return;
    if (sortColumnId !== col.id) {
      setSortColumnId(col.id);
      setSortDir("asc");
      return;
    }
    if (sortDir === "asc") {
      setSortDir("desc");
      return;
    }
    setSortColumnId(null);
    setSortDir("asc");
  }

  const activeFilters = Object.values(filters).some((v) => v.trim());

  if (rows.length === 0 && !activeFilters && emptyPresentation === "message") {
    return (
      <div className={cn("flex min-h-0 flex-1 flex-col", className)}>
        <p className="py-8 text-center text-sm text-muted-foreground">{emptyMessage}</p>
        {footer ? <div className="shrink-0">{footer}</div> : null}
      </div>
    );
  }

  return (
    <div className={cn("flex min-h-0 flex-1 flex-col gap-2", className)}>
      <div
        className={cn(
          "relative min-h-0 flex-1 overflow-hidden",
          framed && "rounded-md border bg-card"
        )}
      >
        <div className="absolute inset-0 overflow-auto overscroll-contain">
          <Table className="border-0" containerClassName="overflow-visible">
            <TableHeader className="sticky top-0 z-10 bg-card shadow-[0_1px_0_0_var(--border)]">
              <TableRow className="hover:bg-transparent">
                {columns.map((col) => (
                  <TableHead key={col.id} className={col.className}>
                    {columnSortable(col) ? (
                      <button
                        type="button"
                        onClick={() => toggleSort(col)}
                        className="inline-flex max-w-full items-center gap-1 text-left font-medium hover:text-foreground"
                      >
                        <span className="truncate">{col.header}</span>
                        {sortColumnId === col.id ? (
                          sortDir === "asc" ? (
                            <ArrowUp className="size-3.5 shrink-0 opacity-70" aria-hidden />
                          ) : (
                            <ArrowDown className="size-3.5 shrink-0 opacity-70" aria-hidden />
                          )
                        ) : (
                          <ArrowUpDown className="size-3.5 shrink-0 opacity-40" aria-hidden />
                        )}
                      </button>
                    ) : (
                      col.header
                    )}
                  </TableHead>
                ))}
              </TableRow>
              <TableRow className="hover:bg-transparent border-b">
                {columns.map((col) => (
                  <TableHead key={`${col.id}-filter`} className="py-1.5 font-normal">
                    {col.filter === "select" ? (
                      <select
                        value={filters[col.id] ?? ""}
                        onChange={(e) => setFilter(col.id, e.target.value)}
                        className="h-7 w-full min-w-[4.5rem] max-w-[10rem] rounded-md border border-input bg-background px-1.5 text-xs text-foreground"
                        aria-label={`Filter ${col.id}`}
                      >
                        <option value="">All</option>
                        {(selectOptions[col.id] ?? []).map((opt) => (
                          <option key={opt} value={opt}>
                            {opt}
                          </option>
                        ))}
                      </select>
                    ) : col.filter === "text" ? (
                      <input
                        type="search"
                        value={filters[col.id] ?? ""}
                        onChange={(e) => setFilter(col.id, e.target.value)}
                        placeholder="Filter…"
                        className="h-7 w-full min-w-[4.5rem] max-w-[11rem] rounded-md border border-input bg-background px-1.5 text-xs"
                        aria-label={`Filter ${col.id}`}
                      />
                    ) : null}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {sorted.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columns.length} className="text-center text-muted-foreground">
                    {activeFilters ? "No rows match the filters." : emptyMessage}
                  </TableCell>
                </TableRow>
              ) : (
                pageRows.map((row, index) => (
                  <TableRow
                    key={rowKey(row)}
                    data-state={isRowSelected?.(row) ? "selected" : undefined}
                    className={cn(onRowClick && "cursor-pointer", rowClassName?.(row, index))}
                    onClick={onRowClick ? () => onRowClick(row) : undefined}
                  >
                    {columns.map((col) => (
                      <TableCell key={col.id} className={col.className}>
                        {col.cell(row)}
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>
          {sorted.length === rows.length
            ? `${rows.length} row${rows.length === 1 ? "" : "s"}`
            : `${sorted.length} of ${rows.length} rows`}
          {pageSize != null && pageSize > 0 && sorted.length > 0
            ? ` · page ${safePage + 1} of ${pageCount}`
            : ""}
        </span>
        <div className="flex flex-wrap items-center gap-2">
          {pageSize != null && pageSize > 0 && pageCount > 1 && (
            <>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 text-xs"
                disabled={safePage <= 0}
                onClick={() => setPage((p) => Math.max(0, p - 1))}
              >
                Previous
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 text-xs"
                disabled={safePage >= pageCount - 1}
                onClick={() => setPage((p) => Math.min(pageCount - 1, p + 1))}
              >
                Next
              </Button>
            </>
          )}
          {footer}
        </div>
      </div>
    </div>
  );
}
