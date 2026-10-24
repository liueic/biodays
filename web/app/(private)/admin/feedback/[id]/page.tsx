import { Mail } from "@react95/icons/Mail";
import type { Metadata } from "next";

import { FeedbackDetailView } from "@/components/admin/feedback-detail";
import { Window } from "@/components/win/window";

export const metadata: Metadata = {
  title: "反馈详情",
};

export default async function AdminFeedbackDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <main>
      <Window
        title="反馈详情 - biodays"
        icon={<Mail variant="16x16_4" aria-hidden />}
        status={["管理员", id]}
      >
        <FeedbackDetailView id={id} />
      </Window>
    </main>
  );
}
