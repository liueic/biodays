"use client";

import { FileFind } from "@react95/icons/FileFind";
import { FileText } from "@react95/icons/FileText";
import { Folder } from "@react95/icons/Folder";
import { HelpBook } from "@react95/icons/HelpBook";
import { Mail } from "@react95/icons/Mail";
import { useRouter } from "next/navigation";
import { type ReactElement, useState } from "react";

import { useDesktopDrag } from "@/components/win/use-desktop-drag";
import { MAIN_WINDOW, openWindow } from "@/components/win/window-store";

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
 * Win95-style desktop icons. Single click selects; clicking the selected icon
 * again (which a double click naturally does) opens it. Enter behaves the same.
 */
export function DesktopIcons({ className }: DesktopIconsProps) {
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
      icon: <FileText variant="32x32_4" width={32} height={32} aria-hidden />,
      onOpen: () => navigate("/"),
      hint: "回到今日故事",
    },
    {
      id: "archive",
      label: "归档浏览",
      icon: <Folder variant="32x32_4" width={32} height={32} aria-hidden />,
      onOpen: () => navigate("/archive"),
      hint: "浏览完整故事归档",
    },
    {
      id: "lab",
      label: "实验室手册",
      icon: <HelpBook variant="32x32_4" width={32} height={32} aria-hidden />,
      onOpen: () => navigate("/lab"),
      hint: "浏览技术解读、实验 Protocol 与实验室技巧",
    },
    {
      id: "feedback",
      label: "灵感信箱",
      icon: <Mail variant="32x32_4" width={32} height={32} aria-hidden />,
      // Cross-group target: full page load so the recorder unloads.
      onOpen: () => navigate("/feedback"),
      hint: "匿名提交反馈、资源或纠错",
    },
    {
      id: "about",
      label: "关于本站",
      icon: <FileFind variant="32x32_4" width={32} height={32} aria-hidden />,
      onOpen: () => navigate("/about"),
      hint: "了解 biodays 与数据来源",
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
      onClick={(event) => {
        if (wasDragged()) return;
        if (selected || event.detail === 0) onOpen();
        else onSelect();
      }}
    >
      <span className="desktop-icon-img">{item.icon}</span>
      <span className="desktop-icon-label">{item.label}</span>
    </button>
  );
}
