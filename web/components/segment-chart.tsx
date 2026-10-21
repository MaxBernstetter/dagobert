"use client";

import { useEffect, useRef, useState } from "react";

const COLOR: Record<string, string> = {
  g: "#22c55e",
  y: "#eab308",
  r: "#ef4444",
};

const STEP = 10_800_000;
const GAP = 3;
const MIN_SLOT = 6;
const MAX_SLOT = 28;

export function SegmentChart({ start, cells }: { start: number; cells: string }) {
  const wrap = useRef<HTMLDivElement>(null);
  const canvas = useRef<HTMLCanvasElement>(null);
  const [slot, setSlot] = useState(10);
  const [offset, setOffset] = useState(0);
  const [tip, setTip] = useState("");
  const drag = useRef<{ x: number; offset: number } | null>(null);

  const pitch = slot + GAP;

  useEffect(() => {
    const node = wrap.current;
    const board = canvas.current;
    if (!node || !board || cells.length === 0) return;
    const width = node.clientWidth;
    board.width = width;
    board.height = 36;
    const visible = Math.max(1, Math.floor(width / pitch));
    const startAt = Math.min(offset, Math.max(0, cells.length - visible));
    const ctx = board.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, width, 36);
    for (let i = 0; i < visible && startAt + i < cells.length; i++) {
      ctx.fillStyle = COLOR[cells[startAt + i]] ?? COLOR.r;
      ctx.fillRect(i * pitch, 4, slot, 28);
    }
  }, [cells, slot, offset, pitch]);

  useEffect(() => {
    const node = wrap.current;
    if (!node || cells.length === 0) return;
    const visible = Math.max(1, Math.floor(node.clientWidth / (10 + GAP)));
    setOffset(Math.max(0, cells.length - visible));
  }, [cells]);

  function visibleCount() {
    const node = wrap.current;
    if (!node) return 1;
    return Math.max(1, Math.floor(node.clientWidth / pitch));
  }

  function onWheel(event: React.WheelEvent) {
    event.preventDefault();
    const node = wrap.current;
    if (!node || cells.length === 0) return;
    const rect = node.getBoundingClientRect();
    const index = offset + (event.clientX - rect.left) / pitch;
    const next = Math.min(MAX_SLOT, Math.max(MIN_SLOT, slot + (event.deltaY < 0 ? 2 : -2)));
    const nextPitch = next + GAP;
    const nextVisible = Math.max(1, Math.floor(node.clientWidth / nextPitch));
    const nextOffset = Math.round(index - (event.clientX - rect.left) / nextPitch);
    setSlot(next);
    setOffset(Math.min(Math.max(0, cells.length - nextVisible), Math.max(0, nextOffset)));
  }

  function hover(clientX: number) {
    const node = wrap.current;
    if (!node || cells.length === 0) return;
    const rect = node.getBoundingClientRect();
    const i = offset + Math.floor((clientX - rect.left) / pitch);
    if (i < 0 || i >= cells.length) {
      setTip("");
      return;
    }
    const when = new Date(start + i * STEP).toISOString().slice(0, 16).replace("T", " ");
    const name = cells[i] === "g" ? "live" : cells[i] === "y" ? "backfill" : "gap";
    setTip(`${when} UTC · ${name}`);
  }

  function onPointerDown(event: React.PointerEvent) {
    drag.current = { x: event.clientX, offset };
    event.currentTarget.setPointerCapture(event.pointerId);
  }

  function onPointerMove(event: React.PointerEvent) {
    if (!drag.current) {
      hover(event.clientX);
      return;
    }
    const delta = Math.round((drag.current.x - event.clientX) / pitch);
    const max = Math.max(0, cells.length - visibleCount());
    setOffset(Math.min(max, Math.max(0, drag.current.offset + delta)));
  }

  if (cells.length === 0) {
    return <p className="text-sm text-muted-foreground">No stored hours for this position.</p>;
  }

  const shown = visibleCount();
  const from = offset + 1;
  const to = Math.min(cells.length, offset + shown);

  return (
    <div className="space-y-2">
      <div
        ref={wrap}
        className="cursor-grab overflow-hidden rounded-md border active:cursor-grabbing"
        onWheel={onWheel}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={() => {
          drag.current = null;
        }}
        onPointerLeave={() => setTip("")}
      >
        <canvas ref={canvas} className="block h-9 w-full" />
      </div>
      <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
        <span className="inline-block size-2 rounded-sm bg-[#22c55e]" />
        <span>Live</span>
        <span className="inline-block size-2 rounded-sm bg-[#eab308]" />
        <span>Backfill</span>
        <span className="inline-block size-2 rounded-sm bg-[#ef4444]" />
        <span>Gap</span>
        <span>
          Slots {from}–{to} of {cells.length}. Drag to move. Wheel zooms.
        </span>
        <span>{tip}</span>
      </div>
    </div>
  );
}
