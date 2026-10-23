"use client";

import { useSyncExternalStore } from "react";

/**
 * Which desktop windows the user has closed. A closed window is hidden (and leaves the
 * taskbar) until something reopens it: a desktop icon, the Start menu, or navigating.
 */
const closed = new Set<string>();
const listeners = new Set<() => void>();
let snapshot: ReadonlySet<string> = new Set();

function emit() {
  snapshot = new Set(closed);
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export const MAIN_WINDOW = "main-window";
export const RECENT_WINDOW = "recent-window";

export function closeWindow(id: string) {
  if (!closed.has(id)) {
    closed.add(id);
    emit();
  }
}

export function openWindow(id: string) {
  if (closed.delete(id)) emit();
}

export function useIsClosed(id: string) {
  return useSyncExternalStore(
    subscribe,
    () => snapshot.has(id),
    () => false,
  );
}
