import { Mail } from "@react95/icons/Mail";

import { Window } from "@/components/win/window";

export default function PrivateLoading() {
  return (
    <main>
      <Window
        title="正在加载 - biodays"
        icon={<Mail variant="16x16_4" aria-hidden />}
        status={["加载中"]}
      >
        <p role="status">正在加载…</p>
      </Window>
    </main>
  );
}
