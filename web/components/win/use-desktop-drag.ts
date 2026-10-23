"use client";

import { useDraggable } from "@neodrag/react";
import { type RefObject, useEffect, useRef, useState } from "react";

/** Where each draggable thing was dropped; survives client-side navigation (not a reload). */
const positions = new Map<string, { x: number; y: number }>();

/** Windows start above the icons; every click on a window raises it above the others. */
let topZ = 20;

function bringToFront(element: HTMLElement) {
  topZ += 1;
  element.style.zIndex = String(topZ);
}

const DESKTOP_QUERY = "(min-width: 768px)";

function useIsDesktop() {
  const [matches, setMatches] = useState(false);
  useEffect(() => {
    const query = window.matchMedia(DESKTOP_QUERY);
    const update = () => setMatches(query.matches);
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);
  return matches;
}

type Options = {
  /** Stable key used to remember the dropped position. */
  id: string;
  /** CSS selector of the element that starts a drag (default: the whole element). */
  handle?: string;
  /** CSS selector of descendants that must not start a drag. */
  cancel?: string;
  /** Snap to a grid, like Win98's "line up icons". */
  grid?: [number, number];
  /** Raise the element above other windows when it is pressed (windows yes, icons no). */
  raise?: boolean;
  /** Temporarily turn dragging off (e.g. while a window is maximized). */
  disabled?: boolean;
};

/**
 * Makes an element draggable on the desktop (≥768px; narrower screens keep the stacked layout).
 * Returns a ref for the element and a `wasDragged()` check so click handlers can ignore the
 * click that browsers fire at the end of a drag.
 */
export function useDesktopDrag<T extends HTMLElement>({
  id,
  handle,
  cancel,
  grid,
  raise = true,
  disabled = false,
}: Options) {
  const ref = useRef<T>(null) as RefObject<T>;
  const dragging = useRef(false);
  const enabled = useIsDesktop();

  useDraggable(ref, {
    handle,
    cancel,
    grid,
    bounds: "body",
    disabled: !enabled || disabled,
    defaultPosition: positions.get(id) ?? { x: 0, y: 0 },
    threshold: { distance: 4 },
    onDragStart: () => {
      dragging.current = true;
      if (raise && ref.current) bringToFront(ref.current);
    },
    onDragEnd: ({ offsetX, offsetY }) => {
      positions.set(id, { x: offsetX, y: offsetY });
      // The click event follows pointerup synchronously; clear the flag right after it.
      setTimeout(() => {
        dragging.current = false;
      }, 0);
    },
  });

  useEffect(() => {
    const element = ref.current;
    if (!raise || !element) return;
    const onPress = () => bringToFront(element);
    element.addEventListener("pointerdown", onPress);
    return () => element.removeEventListener("pointerdown", onPress);
  }, [raise]);

  return { ref, wasDragged: () => dragging.current };
}
