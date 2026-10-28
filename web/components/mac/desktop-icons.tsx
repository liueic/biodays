"use client";

import { useRouter } from "next/navigation";
import { type ReactElement, useState } from "react";

import {
  MacIconBook,
  MacIconDoc,
  MacIconFolderColor,
  MacIconHappyMac,
  MacIconMail,
} from "@/components/mac/icons";
import { useDesktopDrag } from "@/components/win/use-desktop-drag";
import { MAIN_WINDOW, openWindow } from "@/components/win/window-store";
import { cn } from "@/lib/utils";

type DesktopIconsProps = {
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
export function MacDesktopIcons({ className }: DesktopIconsProps) {
  const router = useRouter();
  const [selected, setSelected] = useState<string | null>(null);
  function navigate(path: string) {
    openWindow(MAIN_WINDOW);
    if (path === "/feedback") window.location.assign(path);
    else router.push(path);
  }

  const icons: IconDef[] = [
    {
      id: "today",
      label: "今日故事",
      icon: <MacIconDoc aria-hidden />,
      onOpen: () => navigate("/"),
      hint: "回到今日故事",
    },
    {
      id: "archive",
      label: "归档浏览",
      icon: <MacIconFolderColor aria-hidden />,
      onOpen: () => navigate("/archive"),
      hint: "浏览完整故事归档",
    },
    {
      id: "lab",
      label: "实验室手册",
      icon: <MacIconBook aria-hidden />,
      onOpen: () => navigate("/lab"),
      hint: "浏览技术解读、实验 Protocol 与实验室技巧",
    },
    {
      id: "feedback",
      label: "灵感信箱",
      icon: <MacIconMail aria-hidden />,
      onOpen: () => navigate("/feedback"),
      hint: "匿名提交反馈、资源或纠错",
    },
    {
      id: "about",
      label: "关于本站",
      icon: <MacIconHappyMac aria-hidden />,
      onOpen: () => navigate("/about"),
      hint: "了解 biodays 与数据来源",
    },
  ];

  return (
    <nav aria-label="桌面图标" className={cn("mac-desktop-icons", className)}>
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
      title={`${item.hint}（双击打开，可拖动）`}
      onClick={(event) => {
        if (wasDragged()) return;
        if (selected || event.detail === 0) onOpen();
        else onSelect();
      }}
    >
      <span className="mac-desktop-icon-img">{item.icon}</span>
      <span className="mac-desktop-icon-label">{item.label}</span>
    </button>
  );
}
