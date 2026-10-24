import { Mailnews2 } from "@react95/icons/Mailnews2";
import type { Metadata } from "next";

import { FeedbackInbox } from "@/components/admin/feedback-inbox";
import { Window } from "@/components/win/window";

export const metadata: Metadata = {
  title: "反馈收件箱",
};

export default function AdminFeedbackPage() {
  return (
    <main>
      <Window
        title="反馈收件箱 - Outlook Express"
        icon={<Mailnews2 variant="16x16_4" aria-hidden />}
        status={["管理员", "biodays"]}
        variant="mail"
        width="wide"
      >
        <h1 className="sr-only">反馈收件箱</h1>
        <FeedbackInbox />
      </Window>
    </main>
  );
}
