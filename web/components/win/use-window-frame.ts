"use client";

import { type ReactElement, useEffect, useRef, useState } from "react";

import { useDesktopDrag } from "@/components/win/use-desktop-drag";
import {
  closeWindow,
  focusWindow,
  minimizeWindow,
  openWindow,
  registerWindow,
  useIsClosed,
  useWindows,
} from "@/components/win/window-store";

type Options = {
  /** Stable id shared with the taskbar button and the window store. */
  id: string;
  title: string;
  icon: ReactElement;
  /** Opening a page counts as opening its window, so a closed window comes back. */
  reopenOnMount?: boolean;
};

/**
 * Win98 window behaviour: minimize to a taskbar button, maximize to fill the desktop,
 * close (hide), focus tracking, and dragging by the title bar.
 */
export function useWindowFrame<T extends HTMLElement>({
  id,
  title,
  icon,
  reopenOnMount = false,
}: Options) {
  const closed = useIsClosed(id);
  const desktop = useWindows();
  const minimized = desktop.windows.find((item) => item.id === id)?.minimized ?? false;
  const [maximized, setMaximized] = useState(false);
  const active = desktop.activeWindow === id;
  const iconRef = useRef(icon);
  iconRef.current = icon;

  const { ref } = useDesktopDrag<T>({
    id,
    handle: ".window-drag-handle",
    cancel: "button, a",
    disabled: maximized || minimized || closed,
  });

  useEffect(() => {
    if (reopenOnMount) openWindow(id);
  }, [id, reopenOnMount]);

  // Register with the taskbar while the window is open.
  useEffect(() => {
    if (closed) return;
    return registerWindow(id, title, iconRef.current);
  }, [closed, id, title]);

  return {
    ref,
    active,
    closed,
    minimized,
    maximized,
    /** Hide while closed or minimized. */
    hidden: closed || minimized,
    minimize: () => minimizeWindow(id),
    toggleMaximize: () => setMaximized((value) => !value),
    close: () => closeWindow(id),
    /** Spread onto the window frame so pressing it focuses it in the taskbar. */
    focusProps: { onPointerDownCapture: () => focusWindow(id) },
  };
}
