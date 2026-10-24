import { Mail } from "@react95/icons/Mail";
import type { Metadata } from "next";
import { Suspense } from "react";

import { FeedbackForm } from "@/components/feedback/feedback-form";
import { Window } from "@/components/win/window";

export const metadata: Metadata = {
  title: "灵感信箱",
  description: "匿名提交想看的话题、趣味资源、功能建议或问题纠错，无需注册。",
  alternates: { canonical: "/feedback" },
};

export default function FeedbackPage() {
  return (
    <main>
      <Window
        title="写邮件 - 灵感信箱"
        icon={<Mail variant="16x16_4" aria-hidden />}
        status={["新邮件", "匿名可发送"]}
        variant="mail"
      >
        <h1 className="sr-only">写邮件到灵感信箱</h1>
        <Suspense fallback={<p role="status">正在加载写邮件窗口…</p>}>
          <FeedbackForm />
        </Suspense>
      </Window>
    </main>
  );
}
