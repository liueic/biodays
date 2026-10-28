"use client";

import { Bulb } from "@react95/icons/Bulb";

import { ErrorState } from "@/components/error-state";
import { TodayStory } from "@/components/today-story";
import { RecentWindow } from "@/components/win/recent-window";
import { Window } from "@/components/win/window";
import { RECENT_WINDOW, useIsClosed } from "@/components/win/window-store";
import { useDesktopMode } from "@/hooks/use-desktop-mode";
import type { StorySummary, TodayResponse } from "@/lib/api";
import type { TodayStamp } from "@/lib/date";
import type { DailyLabRecommendations } from "@/lib/lab-recommendations";

type HomeDesktopProps = {
  today: TodayResponse | null;
  recent: StorySummary[];
  stamp: TodayStamp;
  labRecommendations: DailyLabRecommendations;
};

/**
 * A single-screen desktop on large, precise-pointer devices. Smaller screens use
 * one document window and keep all destinations in the system menus.
 */
export function HomeDesktop({ today, recent, stamp, labRecommendations }: HomeDesktopProps) {
  const recentClosed = useIsClosed(RECENT_WINDOW);
  const showPreview = useDesktopMode(1200);

  return (
    <main
      className="home-desktop relative mx-auto w-full max-w-[1120px]"
      data-preview={showPreview && !recentClosed && recent.length > 0 ? "" : undefined}
    >
      <div className="home-main-window mx-auto w-full max-w-[640px]">
        {today ? (
          <TodayStory
            initialStory={today.data}
            isFallback={today.meta.is_fallback}
            stamp={stamp}
            labRecommendations={labRecommendations}
          />
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

      {showPreview && !recentClosed && recent.length > 0 && (
        <RecentWindow
          stories={recent}
          className="home-recent-window relative z-10 mx-auto mt-4 w-full max-w-[640px]"
        />
      )}
    </main>
  );
}
