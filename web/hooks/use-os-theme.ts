"use client";

import { useSyncExternalStore } from "react";

import { detectOsTheme, OS_COOKIE, type OsTheme } from "@/lib/os-theme";

const CHANGE_EVENT = "biodays:os-theme";

function subscribe(onChange: () => void) {
  // Development remounts can clear attributes added by the head script.
  if (!document.documentElement.dataset.os) {
    let theme = detectOsTheme(navigator.userAgent, navigator.maxTouchPoints);
    try {
      const saved = document.cookie.match(/(?:^|;)\s*biodays\.os=(win|mac)(?:;|$)/)?.[1];
      if (saved === "win" || saved === "mac") theme = saved;
    } catch {
      // OS detection still works when cookies are blocked.
    }
    document.documentElement.dataset.os = theme;
  }
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["data-os"],
  });
  window.addEventListener(CHANGE_EVENT, onChange);
  return () => {
    observer.disconnect();
    window.removeEventListener(CHANGE_EVENT, onChange);
  };
}

function getSnapshot(): OsTheme {
  return document.documentElement.dataset.os === "mac" ? "mac" : "win";
}

/** Stable React hydration snapshot; CSS already shows the detected skin before hydration. */
function getServerSnapshot(): OsTheme {
  return "win";
}

/** Persist the choice for future visits and notify every subscribed component. */
export function setOsTheme(theme: OsTheme) {
  document.documentElement.dataset.os = theme;
  try {
    // biome-ignore lint/suspicious/noDocumentCookie: single ASCII enum value; nothing to escape, and this must stay synchronous with the dataset write.
    document.cookie = `${OS_COOKIE}=${theme};path=/;max-age=31536000;SameSite=Lax${location.protocol === "https:" ? ";Secure" : ""}`;
  } catch {
    // Cookies blocked — the in-memory switch still applies to this session.
  }
  window.dispatchEvent(new Event(CHANGE_EVENT));
}

/**
 * Current OS theme. During SSR and the first hydration render this reports "win"
 * (matching the server HTML); it re-renders with the real theme immediately after.
 */
export function useOsTheme(): OsTheme {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
