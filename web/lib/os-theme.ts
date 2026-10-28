/**
 * OS theme ("win" = Windows 98 chrome, "mac" = Mac OS 8/9 Platinum chrome).
 *
 * The active theme lives on `<html data-os="…">`. It is first set by a blocking
 * inline script before first paint (cookie override, else user-agent sniff), so
 * pages stay statically renderable and there is no theme flash. Client code reads
 * it through `useOsTheme()` and writes it through `setOsTheme()` (cookie + dataset).
 */
export type OsTheme = "win" | "mac";

export const OS_COOKIE = "biodays.os";

export function detectOsTheme(userAgent: string, maxTouchPoints = 0): OsTheme {
  const mac =
    /Macintosh|Mac OS X/.test(userAgent) &&
    !/Windows|iPhone|iPad|iPod|Mobile/.test(userAgent) &&
    maxTouchPoints <= 1;
  return mac ? "mac" : "win";
}

/**
 * Source of the inline script injected by `OsThemeScript`. Must stay dependency-free
 * and synchronous; keep the cookie name in sync with `OS_COOKIE`.
 */
export const OS_THEME_INLINE_SCRIPT = `(function(){var os=(/Macintosh|Mac OS X/.test(navigator.userAgent)&&!/Windows|iPhone|iPad|iPod|Mobile/.test(navigator.userAgent)&&(navigator.maxTouchPoints||0)<=1)?"mac":"win";try{var m=document.cookie.match(/(?:^|;)\\s*biodays\\.os=(win|mac)(?:;|$)/);if(m)os=m[1];}catch(e){}document.documentElement.dataset.os=os;})();`;
