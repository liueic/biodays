"use client";

import { type ReactElement, useSyncExternalStore } from "react";

/**
 * Which desktop windows the user has closed. A closed window is hidden (and leaves the
 * taskbar) until something reopens it: a desktop icon, the Start menu, or navigating.
 *
 * The server cannot know this set — it lives in the browser (localStorage). The server
 * snapshot must therefore claim "nothing is closed": rendering the server HTML with the
 * window hidden and the client HTML with it visible is a hydration mismatch, and a server
 * snapshot of "closed" would ship empty pages that no crawler could read.
 */
const STORAGE_KEY = "biodays.closed-windows";

const listeners = new Set<() => void>();
const closed = new Set<string>(readClosed());
let snapshot: ReadonlySet<string> = new Set(closed);
type DesktopWindow = { id: string; title: string; icon: ReactElement; minimized: boolean };
const windows = new Map<string, DesktopWindow>();
let activeWindow: string | null = null;
const serverWindows = { windows: [] as DesktopWindow[], activeWindow: null as string | null };
let windowSnapshot = serverWindows;

function readClosed(): string[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? parsed.filter((id): id is string => typeof id === "string") : [];
  } catch {
    // Private mode / corrupted entry: fall back to "everything open".
    return [];
  }
}

function persist() {
  if (typeof window === "undefined") return;
  try {
    if (closed.size === 0) window.localStorage.removeItem(STORAGE_KEY);
    else window.localStorage.setItem(STORAGE_KEY, JSON.stringify([...closed]));
  } catch {
    // Storage full or blocked — the in-memory state still works for this session.
  }
}

function emit() {
  snapshot = new Set(closed);
  windowSnapshot = { windows: [...windows.values()], activeWindow };
  persist();
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
  else {
    const item = windows.get(id);
    if (!item) return;
    windows.set(id, { ...item, minimized: false });
    activeWindow = id;
    emit();
  }
}

export function registerWindow(id: string, title: string, icon: ReactElement) {
  windows.set(id, { id, title, icon, minimized: false });
  activeWindow = id;
  emit();
  return () => {
    windows.delete(id);
    if (activeWindow === id) activeWindow = null;
    emit();
  };
}

export function focusWindow(id: string) {
  if (activeWindow !== id) {
    activeWindow = id;
    emit();
  }
}

export function minimizeWindow(id: string) {
  const item = windows.get(id);
  if (!item) return;
  windows.set(id, { ...item, minimized: true });
  if (activeWindow === id) activeWindow = null;
  emit();
}

export function toggleWindow(id: string) {
  const item = windows.get(id);
  if (!item) return;
  if (activeWindow === id && !item.minimized) minimizeWindow(id);
  else {
    windows.set(id, { ...item, minimized: false });
    activeWindow = id;
    emit();
  }
}

export function useWindows() {
  return useSyncExternalStore(
    subscribe,
    () => windowSnapshot,
    () => serverWindows,
  );
}

export function useIsClosed(id: string) {
  return useSyncExternalStore(
    subscribe,
    () => snapshot.has(id),
    () => false,
  );
}
