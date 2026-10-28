import { MacMenuBar } from "@/components/mac/menu-bar";
import { DesktopTaskbar } from "@/components/win/taskbar";

/** CSS chooses the chrome before hydration, using the head script's data-os. */
export function OsChrome() {
  return (
    <>
      <div data-os-only="win">
        <DesktopTaskbar />
      </div>
      <div data-os-only="mac">
        <MacMenuBar />
      </div>
    </>
  );
}
