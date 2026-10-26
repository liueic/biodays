import { FileFind2 } from "@react95/icons/FileFind2";

import { ErrorState } from "@/components/error-state";
import { Window } from "@/components/win/window";

export default function NotFound() {
  return (
    <main>
      <Window
        title="未找到页面 - biodays"
        icon={<FileFind2 variant="16x16_4" aria-hidden />}
        status={["404"]}
      >
        <ErrorState
          code="404"
          variant="warning"
          title="没有找到这个页面"
          description="链接可能有误，或该条目尚未收录。"
          action={{ href: "/archive", label: "浏览归档" }}
        />
      </Window>
    </main>
  );
}
