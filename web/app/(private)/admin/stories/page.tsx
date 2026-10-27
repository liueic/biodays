import { Bookmark } from "@react95/icons/Bookmark";
import type { Metadata } from "next";

import { Window } from "@/components/win/window";

export const metadata: Metadata = {
  title: "故事管理",
};

export default function AdminStoriesPage() {
  return (
    <main>
      <Window
        title="故事管理 - biodays"
        icon={<Bookmark variant="16x16_4" aria-hidden />}
        status={["管理员", "biodays"]}
        variant="mail"
        width="wide"
      >
        <h1 className="sr-only">故事管理</h1>
        <p className="reading">
          故事通过 Markdown 和 Git 发布。修改后提交 Pull Request，预览确认后合并发布。
        </p>
        <p className="reading mt-4">
          <a
            className="text-link"
            href="https://github.com/liueic/biodays/tree/main/web/content/stories"
            target="_blank"
            rel="noreferrer"
          >
            打开故事内容仓库
          </a>
        </p>
      </Window>
    </main>
  );
}
