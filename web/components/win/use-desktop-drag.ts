"use client";

import { useDraggable } from "@neodrag/react";
import { type RefObject, useEffect, useRef, useState } from "react";

/** Where each draggable thing was dropped; survives client-side navigation (not a reload). */
const positions = new Map<string, { x: number; y: number }>();

/**
 * Start from the top-left corner during the server render. The drag library reads the
 * starting position while the module is evaluated, and the server has no idea where the
 * user left things — anything else would be a hydration mismatch.
 */
const ORIGIN = { x: 0, y: 0 };

/** Windows start above the icons; every click on a window raises it above the others. */
let topZ = 20;

function bringToFront(element: HTMLElement) {
  topZ += 1;
  element.style.zIndex = String(topZ);
}

const DESKTOP_QUERY = "(min-width: 768px) and (hover: hover) and (pointer: fine)";

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
  /**
   * Classic Mac outline dragging: while dragging, hide the element and move only a
   * dotted outline; the element itself jumps to the drop point on release.
   */
  outline?: boolean;
  /** Temporarily turn dragging off (e.g. while a window is maximized). */
  disabled?: boolean;
};

/**
 * Makes an element draggable with a desktop pointer; touch screens keep native scrolling.
 * Returns a ref for the element and a `wasDragged()` check so click handlers can ignore the
 * click that browsers fire at the end of a drag.
 */
export function useDesktopDrag<T extends HTMLElement>({
  id,
  handle,
  cancel,
  grid,
  raise = true,
  outline = false,
  disabled = false,
}: Options) {
  const ref = useRef<T>(null) as RefObject<T>;
  const dragging = useRef(false);
  // Neodrag keeps its initial event callbacks; read the current skin through a ref.
  const outlineRef = useRef(outline);
  outlineRef.current = outline;
  const ghost = useRef<HTMLDivElement | null>(null);
  const dragStart = useRef(ORIGIN);
  const enabled = useIsDesktop();

  function dropGhost() {
    ghost.current?.remove();
    ghost.current = null;
    if (ref.current) ref.current.style.visibility = "";
  }

  useDraggable(ref, {
    handle,
    cancel,
    grid,
    bounds: "body",
    disabled: !enabled || disabled,
    defaultPosition: typeof window === "undefined" ? ORIGIN : (positions.get(id) ?? ORIGIN),
    threshold: { distance: 4 },
    onDragStart: ({ offsetX, offsetY }) => {
      dragging.current = true;
      if (raise && ref.current) bringToFront(ref.current);
      if (outlineRef.current && ref.current) {
        dragStart.current = { x: offsetX, y: offsetY };
        const rect = ref.current.getBoundingClientRect();
        const layer = document.createElement("div");
        layer.className = "mac-drag-outline";
        layer.style.left = `${rect.left}px`;
        layer.style.top = `${rect.top}px`;
        layer.style.width = `${rect.width}px`;
        layer.style.height = `${rect.height}px`;
        document.body.appendChild(layer);
        ghost.current = layer;
        ref.current.style.visibility = "hidden";
      }
    },
    onDrag: ({ offsetX, offsetY }) => {
      if (ghost.current) {
        ghost.current.style.translate = `${offsetX - dragStart.current.x}px ${offsetY - dragStart.current.y}px`;
      }
    },
    onDragEnd: ({ offsetX, offsetY }) => {
      positions.set(id, { x: offsetX, y: offsetY });
      dropGhost();
      // The click event follows pointerup synchronously; clear the flag right after it.
      setTimeout(() => {
        dragging.current = false;
      }, 0);
    },
  });

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    // Neodrag sets touch-action:none on the entire root even when disabled.
    // Only desktop icons need that; windows restrict dragging to their title bar.
    element.style.touchAction = enabled && !disabled && !handle ? "none" : "auto";
    const position = enabled && !disabled ? (positions.get(id) ?? ORIGIN) : ORIGIN;
    element.style.translate = `${position.x}px ${position.y}px`;
    if (!enabled || disabled) {
      dragging.current = false;
      ghost.current?.remove();
      ghost.current = null;
      element.style.visibility = "";
    }
  }, [enabled, disabled, handle, id]);

  // Remove a leaked outline if the element unmounts mid-drag.
  useEffect(
    () => () => {
      ghost.current?.remove();
      ghost.current = null;
    },
    [],
  );

  useEffect(() => {
    const element = ref.current;
    if (!raise || !element) return;
    const onPress = () => bringToFront(element);
    element.addEventListener("pointerdown", onPress);
    return () => element.removeEventListener("pointerdown", onPress);
  }, [raise]);

  return { ref, wasDragged: () => dragging.current };
}
