"use client";

import { FileFind } from "@react95/icons/FileFind";
import { FileText } from "@react95/icons/FileText";
import { Folder } from "@react95/icons/Folder";
import { HelpBook } from "@react95/icons/HelpBook";
import { Mail } from "@react95/icons/Mail";
import { Star } from "@react95/icons/Star";
import { useRouter } from "next/navigation";
import { type ReactElement, useState } from "react";

import { useDesktopDrag } from "@/components/win/use-desktop-drag";
import { getRandomStory } from "@/lib/api";
import { DOCS_URL } from "@/lib/site";

type DesktopIconsProps = {
  /** Whether the recent-archive preview window is currently open. */
  archiveOpen: boolean;
  /** Reopen the recent-archive preview window after it was closed. */
  onReopenArchive: () => void;
  className?: string;
};

type IconDef = {
  id: string;
  label: string;
  icon: ReactElement;
  onOpen: () => void;
  /** Native tooltip describing what opening this icon does. */
  hint: string;
  disabled?: boolean;
};

/**
 * Win95-style desktop icons. Single click selects; clicking the selected icon
 * again (which a double click naturally does) opens it. Enter behaves the same.
 */
export function DesktopIcons({ archiveOpen, onReopenArchive, className }: DesktopIconsProps) {
  const router = useRouter();
  const [selected, setSelected] = useState<string | null>(null);
  const [randomState, setRandomState] = useState<"idle" | "pending" | "error">("idle");

  async function openRandom() {
    if (randomState === "pending") return;
    setRandomState("pending");
    try {
      const story = await getRandomStory();
      router.push(`/story/${story.id}`);
    } catch {
      setRandomState("error");
      setTimeout(() => setRandomState("idle"), 2000);
      return;
    }
    setRandomState("idle");
  }

  const icons: IconDef[] = [
    {
      id: "today",
      label: "今日故事",
      icon: <FileText variant="32x32_4" width={32} height={32} aria-hidden />,
      onOpen: () => router.push("/"),
      hint: "回到今日故事",
    },
    {
      id: "archive",
      label: "归档浏览",
      icon: <Folder variant="32x32_4" width={32} height={32} aria-hidden />,
      // Preview closed -> reopen it on the desktop; already open -> go to the full archive page.
      onOpen: () => (archiveOpen ? router.push("/archive") : onReopenArchive()),
      hint: archiveOpen ? "打开完整归档页" : "重新打开「最近归档」窗口",
    },
    {
      id: "random",
      label:
        randomState === "pending" ? "请稍候…" : randomState === "error" ? "获取失败！" : "随机一篇",
      icon: <Star variant="32x32_4" width={32} height={32} aria-hidden />,
      onOpen: openRandom,
      hint: "随机打开一篇故事",
      disabled: randomState === "pending",
    },
    {
      id: "feedback",
      label: "灵感信箱",
      icon: <Mail variant="32x32_4" width={32} height={32} aria-hidden />,
      onOpen: () => router.push("/feedback"),
      hint: "匿名提交反馈、资源或纠错",
    },
    {
      id: "about",
      label: "关于本站",
      icon: <FileFind variant="32x32_4" width={32} height={32} aria-hidden />,
      onOpen: () => router.push("/about"),
      hint: "了解 biodays 与数据来源",
    },
    {
      id: "docs",
      label: "API 文档",
      icon: <HelpBook variant="32x32_4" width={32} height={32} aria-hidden />,
      onOpen: () => window.open(DOCS_URL, "_blank", "noopener"),
      hint: "在新标签页打开 Swagger API 文档",
    },
  ];

  return (
    <nav aria-label="桌面图标" className={className}>
      {icons.map((item) => (
        <DesktopIcon
          key={item.id}
          item={item}
          selected={selected === item.id}
          onSelect={() => setSelected(item.id)}
          onOpen={() => {
            setSelected(null);
            item.onOpen();
          }}
        />
      ))}
    </nav>
  );
}

/** One draggable icon. Icons snap to an 8px grid and stay below every window. */
function DesktopIcon({
  item,
  selected,
  onSelect,
  onOpen,
}: {
  item: IconDef;
  selected: boolean;
  onSelect: () => void;
  onOpen: () => void;
}) {
  const { ref, wasDragged } = useDesktopDrag<HTMLButtonElement>({
    id: `icon-${item.id}`,
    grid: [8, 8],
    raise: false,
  });

  return (
    <button
      ref={ref}
      type="button"
      className="desktop-icon w-16 lg:w-[76px]"
      data-selected={selected || undefined}
      disabled={item.disabled}
      title={`${item.hint}（双击打开，可拖动）`}
      onClick={() => {
        if (wasDragged()) return;
        if (selected) onOpen();
        else onSelect();
      }}
    >
      <span className="desktop-icon-img">{item.icon}</span>
      <span className="desktop-icon-label">{item.label}</span>
    </button>
  );
}
