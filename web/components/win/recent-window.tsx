"use client";

import { Button } from "@react95/core/Button";
import { Frame } from "@react95/core/Frame";
import { TitleBar } from "@react95/core/TitleBar";
import { FileText } from "@react95/icons/FileText";
import { Folder } from "@react95/icons/Folder";
import Link from "next/link";

import { useWindowFrame } from "@/components/win/use-window-frame";
import { RECENT_WINDOW } from "@/components/win/window-store";
import type { StorySummary } from "@/lib/api";
import { formatMonthDay } from "@/lib/date";
import { cn } from "@/lib/utils";

type RecentWindowProps = {
  stories: StorySummary[];
  className?: string;
};

/** Small desktop window previewing the latest archived stories. */
export function RecentWindow({ stories, className }: RecentWindowProps) {
  const items = Array.isArray(stories) ? stories.filter(Boolean) : [];
  const frame = useWindowFrame<HTMLDivElement>({
    id: RECENT_WINDOW,
    title: "最近归档",
    icon: <Folder variant="32x32_4" width={16} height={16} aria-hidden />,
  });

  return (
    <Frame
      ref={frame.ref}
      {...frame.focusProps}
      as="section"
      aria-label="最近归档"
      bgColor="$material"
      boxShadow="$out"
      padding="$2"
      className={cn(className, frame.hidden && "hidden")}
    >
      <TitleBar
        active={frame.active}
        title="最近归档"
        className="win98-title window-drag-handle"
        data-inactive={frame.active ? undefined : ""}
        icon={<Folder variant="32x32_4" width={16} height={16} aria-hidden />}
      >
        <TitleBar.OptionsBox>
          <TitleBar.Minimize aria-label="最小化" onClick={frame.minimize} />
          <TitleBar.Close aria-label="关闭" onClick={frame.close} />
        </TitleBar.OptionsBox>
      </TitleBar>

      <Frame
        as="ul"
        boxShadow="$in"
        bgColor="$inputBackground"
        padding="$2"
        className="mt-[2px] list-none"
      >
        {items.map((story) => (
          <li key={story.id}>
            <Link href={`/story/${story.id}`} className="recent-story-link" title={story.title}>
              <FileText variant="16x16_4" aria-hidden className="shrink-0" />
              <span className="whitespace-nowrap">{formatMonthDay(story.month, story.day)}</span>
              <span className="truncate">{story.title}</span>
            </Link>
          </li>
        ))}
      </Frame>

      <div className="mt-2 flex items-center justify-end">
        <Button as={Link} href="/archive" className="px-3">
          全部归档(A)
        </Button>
      </div>
    </Frame>
  );
}
