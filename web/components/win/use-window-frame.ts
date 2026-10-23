"use client";

import { ModalEvents, useModal } from "@react95/core";
import { type ReactElement, useCallback, useEffect, useRef, useState } from "react";

import { useDesktopDrag } from "@/components/win/use-desktop-drag";
import { closeWindow, openWindow, useIsClosed } from "@/components/win/window-store";

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
  const { add, remove, focus, minimize, subscribe } = useModal();
  const closed = useIsClosed(id);
  const [minimized, setMinimized] = useState(false);
  const [maximized, setMaximized] = useState(false);
  const [active, setActive] = useState(false);
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
    add({ id, icon: iconRef.current, title, hasButton: true });
    focus(id);
    return () => remove(id);
  }, [closed, id, title, add, remove, focus]);

  // The taskbar button minimizes/restores through the shared event bus.
  useEffect(() => {
    const offs = [
      subscribe(ModalEvents.MinimizeModal, ({ id: target }: { id?: string }) => {
        if (target === id) setMinimized(true);
      }),
      subscribe(ModalEvents.RestoreModal, ({ id: target }: { id?: string }) => {
        if (target === id) setMinimized(false);
      }),
      subscribe(ModalEvents.ModalVisibilityChanged, ({ id: target }: { id?: string }) => {
        setActive(target === id);
      }),
    ];
    return () => {
      for (const off of offs) off();
    };
  }, [id, subscribe]);

  const minimizeWindow = useCallback(() => {
    minimize(id);
    focus("no-id");
  }, [id, minimize, focus]);

  return {
    ref,
    active,
    closed,
    minimized,
    maximized,
    /** Hide while closed or minimized. */
    hidden: closed || minimized,
    minimize: minimizeWindow,
    toggleMaximize: () => setMaximized((value) => !value),
    close: () => closeWindow(id),
    /** Spread onto the window frame so pressing it focuses it in the taskbar. */
    focusProps: { onPointerDownCapture: () => focus(id) },
  };
}
