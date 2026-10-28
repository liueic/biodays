"use client";

import { useEffect, useState } from "react";

/** Keep pointer behaviour aligned with the desktop media queries in globals.css. */
export function useDesktopMode(minWidth = 1024) {
  const [desktop, setDesktop] = useState(false);
  useEffect(() => {
    const query = window.matchMedia(
      `(min-width: ${minWidth}px) and (hover: hover) and (pointer: fine)`,
    );
    const update = () => setDesktop(query.matches);
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, [minWidth]);
  return desktop;
}
