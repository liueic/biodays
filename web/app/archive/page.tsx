import { Folder } from "@react95/icons";
import type { Metadata } from "next";
import { ArchiveExplorer } from "@/components/archive/archive-explorer";

import { ErrorState } from "@/components/error-state";
import { Window } from "@/components/win/window";
import { getAllStories, getMeta, getStories, getTags } from "@/lib/api";
import { SITE_NAME } from "@/lib/site";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "档案",
  description: "按月份、标签与关键词浏览 biodays 的全部实验室史故事。",
  alternates: { canonical: "/archive" },
  openGraph: {
    title: `档案 · ${SITE_NAME}`,
    description: "按月份、标签与关键词浏览全部实验室史故事。",
    url: "/archive",
    siteName: SITE_NAME,
  },
};

export default async function ArchivePage() {
  let data: Awaited<ReturnType<typeof loadArchive>>;
  try {
    data = await loadArchive();
  } catch {
    return <ArchiveOffline />;
  }

  const { initial, tags, all, meta } = data;
  const monthCounts: Record<number, number> = {};
  for (const story of all) {
    monthCounts[story.month] = (monthCounts[story.month] ?? 0) + 1;
  }

  return (
    <main>
      <Window
        title="归档 - biodays"
        icon={<Folder variant="16x16_4" aria-hidden />}
        status={[`${initial.meta.total} 个对象`, meta ? `覆盖 ${meta.coverage}` : "覆盖范围未知"]}
      >
        <h1 className="sr-only">归档</h1>
        <ArchiveExplorer initial={initial} tags={tags} monthCounts={monthCounts} />
      </Window>
    </main>
  );
}

async function loadArchive() {
  const [initial, tags, all, meta] = await Promise.all([
    getStories({ page: 1, pageSize: 12 }),
    getTags(),
    getAllStories(),
    getMeta().then((response) => response.data),
  ]);
  return { initial, tags: tags.data, all, meta };
}

function ArchiveOffline() {
  return (
    <main>
      <Window
        title="归档 - biodays"
        icon={<Folder variant="16x16_4" aria-hidden />}
        status={["无法连接"]}
      >
        <h1 className="sr-only">归档</h1>
        <ErrorState
          title="归档暂时无法加载"
          description="后端 API 没有响应，请确认服务已启动，或稍后刷新。"
          action={{ href: "/", label: "回到今日" }}
        />
      </Window>
    </main>
  );
}
