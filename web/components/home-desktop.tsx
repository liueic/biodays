"use client";

import { Bulb } from "@react95/icons";
import { useState } from "react";

import { ErrorState } from "@/components/error-state";
import { TodayStory } from "@/components/today-story";
import { DesktopIcons } from "@/components/win/desktop-icons";
import { RecentWindow } from "@/components/win/recent-window";
import { Window } from "@/components/win/window";
import type { Story, TodayResponse } from "@/lib/api";
import type { TodayStamp } from "@/lib/date";

type HomeDesktopProps = {
  today: TodayResponse | null;
  recent: Story[];
  stamp: TodayStamp;
};

/**
 * Homepage desktop composition: icons on the left, the main window centered,
 * and a small recent-archive window overlapping its bottom-right corner on wide
 * screens. On narrow screens everything stacks vertically.
 */
export function HomeDesktop({ today, recent, stamp }: HomeDesktopProps) {
  const [recentOpen, setRecentOpen] = useState(true);

  return (
    <main className="relative mx-auto w-full max-w-[1120px]">
      <DesktopIcons
        archiveOpen={recentOpen}
        onReopenArchive={() => setRecentOpen(true)}
        className="flex flex-wrap items-start gap-x-3 gap-y-4 px-1 pb-5 lg:absolute lg:top-0 lg:left-0 lg:flex-col lg:gap-6 lg:p-0"
      />

      <div className="lg:mx-auto lg:max-w-[640px]">
        {today ? (
          <TodayStory initialStory={today.data} isFallback={today.meta.is_fallback} stamp={stamp} />
        ) : (
          <Window
            title="biodays"
            icon={<Bulb variant="32x32_4" width={16} height={16} aria-hidden />}
            status={["就绪"]}
          >
            <ErrorState
              title="暂时无法获取今日故事"
              description="服务似乎没有响应，请稍后刷新重试。"
              action={{ href: "/archive", label: "浏览归档" }}
            />
          </Window>
        )}
      </div>

      {recentOpen && recent.length > 0 && (
        <RecentWindow
          stories={recent}
          onClose={() => setRecentOpen(false)}
          className="relative z-10 mx-auto mt-4 w-full max-w-[640px] lg:mx-0 lg:mr-4 lg:ml-auto lg:w-[340px] lg:-mt-16"
        />
      )}
    </main>
  );
}
