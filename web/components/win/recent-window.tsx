"use client";

import { Button, Frame, TitleBar } from "@react95/core";
import { FileText, Folder } from "@react95/icons";
import Link from "next/link";

import { useDesktopDrag } from "@/components/win/use-desktop-drag";
import type { Story } from "@/lib/api";
import { formatMonthDay } from "@/lib/date";

type RecentWindowProps = {
  stories: Story[];
  onClose: () => void;
  className?: string;
};

/** Small desktop window previewing the latest archived stories. */
export function RecentWindow({ stories, onClose, className }: RecentWindowProps) {
  const { ref } = useDesktopDrag<HTMLDivElement>({
    id: "recent-window",
    handle: ".window-drag-handle",
    cancel: "button, a",
  });

  return (
    <Frame
      ref={ref}
      as="section"
      aria-label="最近归档"
      bgColor="$material"
      boxShadow="$out"
      padding="$2"
      className={className}
    >
      <TitleBar
        active
        title="最近归档"
        className="win98-title window-drag-handle"
        icon={<Folder variant="32x32_4" width={16} height={16} aria-hidden />}
      >
        <TitleBar.OptionsBox>
          <TitleBar.Close aria-label="关闭最近归档窗口" onClick={onClose} />
        </TitleBar.OptionsBox>
      </TitleBar>

      <Frame
        as="ul"
        boxShadow="$in"
        bgColor="$inputBackground"
        padding="$2"
        className="mt-[2px] list-none"
      >
        {stories.map((story) => (
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
