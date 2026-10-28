import { OS_THEME_INLINE_SCRIPT } from "@/lib/os-theme";

/**
 * Blocking inline script that sets `<html data-os>` before first paint (cookie
 * override, else user-agent sniff). Server-rendering the attribute instead would
 * require `headers()` in the root layout, which would force every route — including
 * the ISR story pages — into dynamic rendering. The `<html>` element already has
 * `suppressHydrationWarning`, so the attribute never causes a hydration mismatch.
 */
export function OsThemeScript() {
  // biome-ignore lint/security/noDangerouslySetInnerHtml: static first-party script, no user data
  return <script dangerouslySetInnerHTML={{ __html: OS_THEME_INLINE_SCRIPT }} />;
}
