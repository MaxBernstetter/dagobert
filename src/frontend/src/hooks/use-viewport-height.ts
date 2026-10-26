
import { useEffect, useState } from "react";

function heightFor(min: number, max: number, share: number): number {
  if (typeof window === "undefined") return min;
  return Math.min(max, Math.max(min, Math.round(window.innerHeight * share)));
}

/** Chart height that follows the window height: `share` of the viewport, clamped to [min, max] px. */
export function useViewportHeight(min: number, max: number, share: number): number {
  const [height, setHeight] = useState(() => heightFor(min, max, share));
  useEffect(() => {
    const onResize = () => setHeight(heightFor(min, max, share));
    onResize();
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [min, max, share]);
  return height;
}
