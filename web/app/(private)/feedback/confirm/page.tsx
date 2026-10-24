import { Mail2 } from "@react95/icons/Mail2";
import type { Metadata } from "next";

import { TokenAction } from "@/components/feedback/token-action";
import { Window } from "@/components/win/window";

export const metadata: Metadata = {
  title: "确认订阅",
};

export default function ConfirmPage() {
  return (
    <main>
      <Window
        title="确认订阅 - biodays"
        icon={<Mail2 variant="16x16_4" aria-hidden />}
        status={["邮件确认", "biodays"]}
      >
        <h1 className="text-base font-bold">确认订阅反馈处理结果</h1>
        <div className="mt-3">
          <TokenAction mode="confirm" />
        </div>
      </Window>
    </main>
  );
}
