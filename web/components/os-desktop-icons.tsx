import type { ComponentProps } from "react";
import { MacDesktopIcons } from "@/components/mac/desktop-icons";
import { DesktopIcons as WinDesktopIcons } from "@/components/win/desktop-icons";

/** Both skins are available in the static HTML; only the selected one is visible. */
export function DesktopIcons(props: ComponentProps<typeof WinDesktopIcons>) {
  return (
    <>
      <div data-os-only="win">
        <WinDesktopIcons {...props} />
      </div>
      <div data-os-only="mac">
        <MacDesktopIcons {...props} />
      </div>
    </>
  );
}
