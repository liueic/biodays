import { Settings } from "@react95/icons/Settings";
import type { Metadata } from "next";

import { SettingsPanel } from "@/components/admin/settings-panel";
import { Window } from "@/components/win/window";

export const metadata: Metadata = {
  title: "站点设置",
};

export default function AdminSettingsPage() {
  return (
    <main>
      <Window
        title="站点设置 - biodays"
        icon={<Settings variant="16x16_4" aria-hidden />}
        status={["设置", "biodays"]}
      >
        <SettingsPanel />
      </Window>
    </main>
  );
}
