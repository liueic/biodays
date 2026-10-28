"use client";

import { Logo } from "@react95/icons/Logo";
import { useRouter } from "next/navigation";
import { type ReactElement, useState } from "react";

import {
  MacIconBook,
  MacIconDoc,
  MacIconFolderColor,
  MacIconHappyMac,
  MacIconMail,
  MacIconSparkle,
  MacIconTrash,
} from "@/components/mac/icons";
import { useDesktopDrag } from "@/components/win/use-desktop-drag";
import { setOsTheme } from "@/hooks/use-os-theme";
import { getRandomStory } from "@/lib/api";
import { DOCS_URL } from "@/lib/site";
import { cn } from "@/lib/utils";

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
 * Classic-Mac desktop icons: black-and-white artwork that inverts when selected,
 * black-on-desktop labels that gain a black highlight plate when selected.
 * Single click selects; clicking the selected icon again (which a double click
 * naturally does) opens it. Icons drag as dotted outlines, like the real Finder.
 */
export function MacDesktopIcons({ archiveOpen, onReopenArchive, className }: DesktopIconsProps) {
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
      icon: <MacIconDoc aria-hidden />,
      onOpen: () => router.push("/"),
      hint: "回到今日故事",
    },
    {
      id: "archive",
      label: "归档浏览",
      icon: <MacIconFolderColor aria-hidden />,
      // Preview closed -> reopen it on the desktop; already open -> go to the full archive page.
      onOpen: () => (archiveOpen ? router.push("/archive") : onReopenArchive()),
      hint: archiveOpen ? "打开完整归档页" : "重新打开「最近归档」窗口",
    },
    {
      id: "random",
      label:
        randomState === "pending" ? "请稍候…" : randomState === "error" ? "获取失败！" : "随机一篇",
      icon: <MacIconSparkle aria-hidden />,
      onOpen: openRandom,
      hint: "随机打开一篇故事",
      disabled: randomState === "pending",
    },
    {
      id: "lab",
      label: "实验室手册",
      icon: <MacIconBook aria-hidden />,
      onOpen: () => router.push("/lab"),
      hint: "浏览技术解读、实验 Protocol 与实验室技巧",
    },
    {
      id: "theme",
      label: "切换到 Win98",
      icon: <Logo variant="32x32_4" width={32} height={32} aria-hidden />,
      onOpen: () => setOsTheme("win"),
      hint: "切换并记住 Windows 98 主题",
    },
    {
      id: "feedback",
      label: "灵感信箱",
      icon: <MacIconMail aria-hidden />,
      onOpen: () => window.location.assign("/feedback"),
      hint: "匿名提交反馈、资源或纠错",
    },
    {
      id: "about",
      label: "关于本站",
      icon: <MacIconHappyMac aria-hidden />,
      onOpen: () => router.push("/about"),
      hint: "了解 biodays 与数据来源",
    },
    {
      id: "docs",
      label: "API 文档",
      icon: <MacIconBook aria-hidden />,
      onOpen: () => window.open(DOCS_URL, "_blank", "noopener"),
      hint: "在新标签页打开 Swagger API 文档",
    },
    {
      id: "mac-trash",
      label: "废纸篓",
      icon: <MacIconTrash aria-hidden />,
      // The Trash is decorative: it just sits in the corner like the real one.
      onOpen: () => {},
      hint: "废纸篓",
    },
  ];

  return (
    <nav aria-label="桌面图标" className={cn("mac-desktop-icons", className)}>
      {icons.map((item) => (
        <DesktopIcon
          key={item.id}
          item={item}
          className={item.id === "mac-trash" ? "mac-desktop-icon-mac-trash" : undefined}
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
  className,
}: {
  item: IconDef;
  selected: boolean;
  onSelect: () => void;
  onOpen: () => void;
  className?: string;
}) {
  const { ref, wasDragged } = useDesktopDrag<HTMLButtonElement>({
    id: `icon-${item.id}`,
    grid: [8, 8],
    raise: false,
    outline: true,
  });

  return (
    <button
      ref={ref}
      type="button"
      className={cn("mac-desktop-icon w-16 lg:w-[76px]", className)}
      data-selected={selected || undefined}
      disabled={item.disabled}
      aria-label={item.id === "theme" ? "切换到 Windows 98" : undefined}
      title={`${item.hint}（双击打开，可拖动）`}
      onClick={() => {
        if (wasDragged()) return;
        if (selected) onOpen();
        else onSelect();
      }}
    >
      <span className="mac-desktop-icon-img">{item.icon}</span>
      <span className="mac-desktop-icon-label">{item.label}</span>
    </button>
  );
}
