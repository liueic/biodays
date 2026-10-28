"use client";

import { useDraggable } from "@neodrag/react";
import { type RefObject, useEffect, useRef, useState } from "react";

import { useDesktopMode } from "@/hooks/use-desktop-mode";

/** Where each draggable thing was dropped; survives client-side navigation (not a reload). */
const positions = new Map<string, { x: number; y: number }>();

/**
 * Start at the layout's origin on the server and on the first client render.
 * Remembered offsets are restored after mounting, within the current workspace.
 */
const ORIGIN = { x: 0, y: 0 };

/** Windows start above the icons; every click on a window raises it above the others. */
let topZ = 20;

export function bringToFront(element: HTMLElement) {
  topZ += 1;
  element.style.zIndex = String(topZ);
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
  const enabled = useDesktopMode();
  const [position, setPosition] = useState(ORIGIN);
  const displayedPosition = enabled && !disabled ? position : ORIGIN;
  const positionRef = useRef(displayedPosition);
  positionRef.current = displayedPosition;
  const reconcilePosition = useRef(() => {});

  function dropGhost() {
    ghost.current?.remove();
    ghost.current = null;
    if (ref.current) ref.current.style.visibility = "";
  }

  useDraggable(ref, {
    handle,
    cancel,
    grid,
    bounds: ".desktop-workspace",
    recomputeBounds: { dragStart: true, drag: true },
    disabled: !enabled || disabled,
    position: displayedPosition,
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
      const next = { x: offsetX, y: offsetY };
      positions.set(id, next);
      setPosition(next);
      if (ghost.current) {
        ghost.current.style.translate = `${offsetX - dragStart.current.x}px ${offsetY - dragStart.current.y}px`;
      }
    },
    onDragEnd: ({ offsetX, offsetY }) => {
      const next = { x: offsetX, y: offsetY };
      positions.set(id, next);
      setPosition(next);
      dropGhost();
      // The click event follows pointerup synchronously; clear the flag right after it.
      setTimeout(() => {
        dragging.current = false;
        // A resize can arrive before pointerup finishes, while the observer is
        // still skipping the active drag. Recheck its bounds once the drag ends.
        reconcilePosition.current();
      }, 0);
    },
  });

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    // Neodrag sets touch-action:none on the entire root even when disabled.
    // Only desktop icons need that; windows restrict dragging to their title bar.
    element.style.touchAction = enabled && !disabled && !handle ? "none" : "auto";
    if (!enabled || disabled) {
      dragging.current = false;
      ghost.current?.remove();
      ghost.current = null;
      element.style.visibility = "";
      return;
    }

    // Reconcile remembered offsets with the current layout after resizing, changing
    // skins, restoring a window, or returning from the compact document mode.
    const workspace = element.closest(".desktop-workspace");
    if (!workspace) return;
    const reconcile = () => {
      if (dragging.current || !element.getClientRects().length) return;
      const rect = element.getBoundingClientRect();
      const bounds = workspace.getBoundingClientRect();
      const originX = rect.left - positionRef.current.x;
      const originY = rect.top - positionRef.current.y;
      const previous = positions.get(id) ?? ORIGIN;
      const minX = bounds.left - originX;
      const minY = bounds.top - originY;
      const next = {
        x: Math.min(
          Math.max(previous.x, minX),
          Math.max(minX, bounds.right - originX - rect.width),
        ),
        y: Math.min(
          Math.max(previous.y, minY),
          Math.max(minY, bounds.bottom - originY - rect.height),
        ),
      };
      positions.set(id, next);
      setPosition((current) => (current.x === next.x && current.y === next.y ? current : next));
    };
    reconcilePosition.current = reconcile;
    // Run after Neodrag applies the controlled translation for this render.
    const frame = requestAnimationFrame(reconcile);
    const observer = new ResizeObserver(reconcile);
    observer.observe(workspace);
    observer.observe(element);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      reconcilePosition.current = () => {};
    };
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
