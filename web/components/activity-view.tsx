"use client";

import { useMemo, useState } from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import type { ActivityRow } from "@/lib/read";
import { StatusBadge, rowStatusLine } from "@/components/status-badge";
import { CandleDetail } from "@/components/candle-detail";

function formatCandleClose(ms: number): string {
  if (!ms) return "—";
  return new Date(ms).toISOString().replace("T", " ").replace(/\.\d{3}Z$/, " UTC");
}

export function ActivityView({ rows }: { rows: ActivityRow[] }) {
  const [selectedId, setSelectedId] = useState<string | null>(
    rows[0]?.orderLinkId ?? null
  );
  const [sheetOpen, setSheetOpen] = useState(false);

  const selected = useMemo(
    () => rows.find((r) => r.orderLinkId === selectedId) ?? null,
    [rows, selectedId]
  );

  function selectRow(id: string) {
    setSelectedId(id);
    if (typeof window !== "undefined" && window.matchMedia("(max-width: 1023px)").matches) {
      setSheetOpen(true);
    }
  }

  return (
    <>
      <div className="grid gap-6 lg:grid-cols-2 lg:items-start">
        <div className="min-w-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Candle</TableHead>
                <TableHead>Symbol</TableHead>
                <TableHead>Rule</TableHead>
                <TableHead>Decision</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-muted-foreground text-center">
                    No activity yet.
                  </TableCell>
                </TableRow>
              ) : (
                rows.map((row, index) => {
                  const isLatest = index === 0;
                  const isSelected = row.orderLinkId === selectedId;
                  return (
                    <TableRow
                      key={row.orderLinkId}
                      data-state={isSelected ? "selected" : undefined}
                      className={`cursor-pointer ${
                        isLatest
                          ? "bg-muted/60 border-l-4 border-l-primary font-medium"
                          : ""
                      } ${isSelected ? "bg-muted" : ""}`}
                      onClick={() => selectRow(row.orderLinkId)}
                    >
                      <TableCell className="whitespace-nowrap text-xs">
                        {formatCandleClose(row.candleClose)}
                      </TableCell>
                      <TableCell>{row.symbol}</TableCell>
                      <TableCell className="max-w-[8rem] truncate font-mono text-xs">
                        {row.rule ?? "—"}
                      </TableCell>
                      <TableCell>{row.decision ?? "—"}</TableCell>
                      <TableCell>
                        <div className="flex flex-col gap-1">
                          <StatusBadge kind={row.kind} />
                          <span className="text-xs text-muted-foreground line-clamp-2">
                            {rowStatusLine(row)}
                          </span>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </div>

        <div className="hidden lg:block min-w-0">
          <CandleDetail row={selected} />
        </div>
      </div>

      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
        <SheetContent side="bottom" className="max-h-[85vh] overflow-y-auto lg:hidden">
          <SheetHeader>
            <SheetTitle>Candle detail</SheetTitle>
          </SheetHeader>
          <div className="mt-4">
            <CandleDetail row={selected} />
          </div>
        </SheetContent>
      </Sheet>
    </>
  );
}
