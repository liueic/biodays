import { OsChrome } from "@/components/os-chrome";
import { Providers } from "@/components/providers";

/**
 * Shared desktop chrome for both OS skins. Both route groups use it, but only the reading
 * layout loads analytics scripts.
 */
export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <Providers>
      <a href="#main" className="skip-link">
        跳到正文
      </a>
      <div
        id="main"
        className="relative z-[1] flex flex-1 flex-col px-2 pt-2 pb-12 sm:px-6 sm:pt-6"
      >
        {/* Spare vertical space is split 0.382 : 0.618 (golden ratio), so short pages sit
            slightly above center; tall pages leave no spare space and just scroll. */}
        <div aria-hidden className="grow-[382] basis-0" />
        <div>{children}</div>
        <div aria-hidden className="grow-[618] basis-0" />
      </div>
      <OsChrome />
    </Providers>
  );
}
