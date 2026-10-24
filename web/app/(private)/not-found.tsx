import { Mail } from "@react95/icons/Mail";

import { ErrorState } from "@/components/error-state";
import { Window } from "@/components/win/window";

export default function PrivateNotFound() {
  return (
    <main>
      <Window
        title="未找到页面 - biodays"
        icon={<Mail variant="16x16_4" aria-hidden />}
        status={["404"]}
      >
        <ErrorState
          code="404"
          title="没有找到这个页面"
          description="链接可能有误，或者该页面已被移动。"
          action={{ href: "/feedback", label: "返回灵感信箱" }}
        />
      </Window>
    </main>
  );
}
