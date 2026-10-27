import { Mailnews3 } from "@react95/icons/Mailnews3";
import type { Metadata } from "next";

import { MailPanel } from "@/components/admin/mail-panel";
import { Window } from "@/components/win/window";

export const metadata: Metadata = {
  title: "邮件状态",
};

export default function AdminMailPage() {
  return (
    <main>
      <Window
        title="邮件状态 - biodays"
        icon={<Mailnews3 variant="16x16_4" aria-hidden />}
        status={["Resend", "biodays"]}
      >
        <MailPanel />
      </Window>
    </main>
  );
}
