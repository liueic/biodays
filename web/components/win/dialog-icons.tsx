import { InfoBubble, Warning } from "@react95/icons";
import type { ReactElement } from "react";

/**
 * Pixel-art dialog icons in the classic Windows 98 message-box style.
 *
 * @react95/icons has the yellow warning triangle and the info bubble, but no
 * red "critical stop" circle, so that one is drawn here from a 16×16 pixel
 * map rendered at 32px with crisp edges.
 */

// o = dark red rim, r = red, h = highlight, w = white cross
const CRITICAL_STOP = [
  "................",
  "......oooo......",
  "....oooooooo....",
  "...oohhhhhhoo...",
  "..oowwrrrrwwoo..",
  "..ohhwwrrwwrro..",
  ".oohhrwwwwrrroo.",
  ".oorrrrwwrrrroo.",
  ".oorrrwwwwrrroo.",
  ".oorrwwrrwwrroo.",
  "..orwwrrrrwwro..",
  "..owwrrrrrrwwo..",
  "...oorrrrrroo...",
  "....oooooooo....",
  "......oooo......",
  "................",
];

const CRITICAL_COLORS: Record<string, string> = {
  o: "#7f0000",
  r: "#d8181d",
  h: "#ff4545",
  w: "#ffffff",
};

/** Merge consecutive same-colour pixels into one rect, row by row. */
function pixelRects(grid: string[], colors: Record<string, string>) {
  const rects: { x: number; y: number; width: number; fill: string }[] = [];
  for (let y = 0; y < grid.length; y++) {
    let x = 0;
    while (x < grid[y].length) {
      const color = grid[y][x];
      if (!colors[color]) {
        x++;
        continue;
      }
      let end = x + 1;
      while (end < grid[y].length && grid[y][end] === color) end++;
      rects.push({ x, y, width: end - x, fill: colors[color] });
      x = end;
    }
  }
  return rects;
}

export function CriticalStopIcon({ size = 32 }: { size?: number }) {
  const rects = pixelRects(CRITICAL_STOP, CRITICAL_COLORS);
  return (
    <svg
      role="presentation"
      width={size}
      height={size}
      viewBox="0 0 16 16"
      shapeRendering="crispEdges"
      className="shrink-0"
    >
      {rects.map((rect) => (
        <rect
          key={`${rect.x}-${rect.y}`}
          x={rect.x}
          y={rect.y}
          width={rect.width}
          height={1}
          fill={rect.fill}
        />
      ))}
    </svg>
  );
}

export type DialogVariant = "critical" | "warning" | "info";

/** The 32px message icon that belongs to each dialog variant. */
export function DialogIcon({ variant }: { variant: DialogVariant }): ReactElement {
  switch (variant) {
    case "warning":
      return <Warning variant="32x32_4" width={32} height={32} aria-hidden className="shrink-0" />;
    case "info":
      return (
        <InfoBubble variant="32x32_4" width={32} height={32} aria-hidden className="shrink-0" />
      );
    default:
      return <CriticalStopIcon size={32} />;
  }
}
