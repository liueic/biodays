import { Mail3 } from "@react95/icons/Mail3";
import type { Metadata } from "next";

import { TokenAction } from "@/components/feedback/token-action";
import { Window } from "@/components/win/window";

export const metadata: Metadata = {
  title: "退订邮件",
};

export default function UnsubscribePage() {
  return (
    <main>
      <Window
        title="退订邮件 - biodays"
        icon={<Mail3 variant="16x16_4" aria-hidden />}
        status={["邮件退订", "biodays"]}
      >
        <h1 className="text-base font-bold">退订反馈邮件</h1>
        <div className="mt-3">
          <TokenAction mode="unsubscribe" />
        </div>
      </Window>
    </main>
  );
}
