"use client";

import { Logo } from "@react95/icons/Logo";
import Link from "next/link";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { MacIconAppleColor, MacIconFinder } from "@/components/mac/icons";
import { closeWindow, MAIN_WINDOW, openWindow, useWindows } from "@/components/win/window-store";
import { setOsTheme } from "@/hooks/use-os-theme";
import { DOCS_URL } from "@/lib/site";
import { cn } from "@/lib/utils";

type MenuId = "apple" | "file" | "edit" | "view" | "special" | "help" | "windows";

type MenuEntry =
  | { kind: "link"; label: string; href: string; external?: boolean; shortcut?: string }
  | { kind: "action"; label: string; onSelect: () => void; shortcut?: string; disabled?: boolean }
  | { kind: "separator" }
  | { kind: "label"; label: string };

/**
 * Classic Macintosh menu bar (System 7.6 / Mac OS 8 era): rainbow Apple menu,
 * File/Edit/View/Special/Help, then the clock and the Finder application menu
 * on the right. Open menus invert to white-on-black and track the pointer
 * across the bar, like the real thing.
 */
export function MacMenuBar() {
  const desktop = useWindows();
  const [openMenu, setOpenMenu] = useState<MenuId | null>(null);
  const [viewMode, setViewMode] = useState<"icon" | "name" | "date">("icon");
  const [clock, setClock] = useState("");
  const ref = useRef<HTMLElement>(null);

  useEffect(() => {
    const update = () =>
      setClock(
        new Date().toLocaleTimeString("en-US", {
          hour: "numeric",
          minute: "2-digit",
          hour12: true,
        }),
      );
    update();
    const timer = window.setInterval(update, 10_000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!openMenu) return;
    const dismiss = (event: PointerEvent) => {
      if (!ref.current?.contains(event.target as Node)) setOpenMenu(null);
    };
    const onEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpenMenu(null);
    };
    document.addEventListener("pointerdown", dismiss);
    document.addEventListener("keydown", onEscape);
    return () => {
      document.removeEventListener("pointerdown", dismiss);
      document.removeEventListener("keydown", onEscape);
    };
  }, [openMenu]);

  const toggleMenu = (menu: MenuId) => setOpenMenu((current) => (current === menu ? null : menu));

  /** Once any menu is open, moving across the bar switches menus without clicking. */
  const trackMenu = (menu: MenuId) => {
    if (openMenu && openMenu !== menu) setOpenMenu(menu);
  };

  const menus: Record<Exclude<MenuId, "windows">, MenuEntry[]> = {
    apple: [
      { kind: "link", label: "关于 biodays…", href: "/about" },
      { kind: "separator" },
      { kind: "action", label: "切换到 Windows 98", onSelect: () => setOsTheme("win") },
    ],
    file: [
      { kind: "link", label: "今日故事", href: "/" },
      { kind: "link", label: "归档浏览", href: "/archive" },
      { kind: "link", label: "实验室手册", href: "/lab" },
      { kind: "link", label: "灵感信箱", href: "/feedback" },
      { kind: "link", label: "关于本站", href: "/about" },
      { kind: "separator" },
      {
        kind: "action",
        label: "关闭窗口",
        shortcut: "⌘W",
        onSelect: () => closeWindow(MAIN_WINDOW),
      },
    ],
    edit: [
      { kind: "label", label: "撤销" },
      { kind: "separator" },
      { kind: "label", label: "剪切" },
      { kind: "label", label: "拷贝" },
      { kind: "label", label: "粘贴" },
      { kind: "label", label: "全选" },
    ],
    view: [
      {
        kind: "action",
        label: `${viewMode === "icon" ? "✓ " : ""}按图标`,
        onSelect: () => setViewMode("icon"),
      },
      {
        kind: "action",
        label: `${viewMode === "name" ? "✓ " : ""}按名称`,
        onSelect: () => setViewMode("name"),
      },
      {
        kind: "action",
        label: `${viewMode === "date" ? "✓ " : ""}按日期`,
        onSelect: () => setViewMode("date"),
      },
    ],
    special: [
      { kind: "label", label: "清空废纸篓" },
      { kind: "separator" },
      { kind: "action", label: "重新启动", onSelect: () => window.location.reload() },
      { kind: "label", label: "关机" },
    ],
    help: [
      { kind: "link", label: "biodays 帮助", href: "/about" },
      { kind: "link", label: "API 文档", href: DOCS_URL, external: true },
    ],
  };

  const editShortcuts = ["⌘Z", "", "⌘X", "⌘C", "⌘V", "⌘A"];

  function renderEntries(id: Exclude<MenuId, "windows">) {
    let shortcutIndex = 0;
    return menus[id].map((entry, index) => {
      const key = `${id}-${index}`;
      if (entry.kind === "separator") {
        return <li key={key} aria-hidden className="mac-menu-separator" />;
      }
      if (entry.kind === "label") {
        // Greyed-out items, like Finder's Edit menu when nothing is selected.
        const shortcut = id === "edit" ? editShortcuts[shortcutIndex++] : undefined;
        return (
          <li key={key} className="mac-menu-item mac-menu-item-disabled" aria-disabled="true">
            <span>{entry.label}</span>
            {shortcut && <span className="mac-menu-shortcut">{shortcut}</span>}
          </li>
        );
      }
      if (entry.kind === "link") {
        // A document navigation unloads the public session recorder.
        const MenuLink = entry.href.startsWith("/feedback") ? "a" : Link;
        return (
          <li key={key}>
            <MenuLink
              href={entry.href}
              className="mac-menu-item"
              target={entry.external ? "_blank" : undefined}
              rel={entry.external ? "noopener" : undefined}
              onClick={() => {
                setOpenMenu(null);
                // Menu navigation reopens the main window, mirroring the Win98 Start menu.
                if (!entry.external) openWindow(MAIN_WINDOW);
              }}
            >
              <span>{entry.label}</span>
              {entry.shortcut && <span className="mac-menu-shortcut">{entry.shortcut}</span>}
            </MenuLink>
          </li>
        );
      }
      return (
        <li key={key}>
          <button
            type="button"
            className="mac-menu-item"
            disabled={entry.disabled}
            onClick={() => {
              setOpenMenu(null);
              entry.onSelect();
            }}
          >
            <span>{entry.label}</span>
            {entry.shortcut && <span className="mac-menu-shortcut">{entry.shortcut}</span>}
          </button>
        </li>
      );
    });
  }

  const barMenuProps = { openMenu, toggleMenu, trackMenu };

  return (
    <header ref={ref} className="mac-menu-bar">
      <div className="mac-menu-bar-left">
        <BarMenu id="apple" label={<MacIconAppleColor aria-hidden />} {...barMenuProps}>
          {renderEntries("apple")}
        </BarMenu>
        <BarMenu id="file" label="File" {...barMenuProps}>
          {renderEntries("file")}
        </BarMenu>
        <BarMenu id="edit" label="Edit" {...barMenuProps}>
          {renderEntries("edit")}
        </BarMenu>
        <BarMenu id="view" label="View" {...barMenuProps}>
          {renderEntries("view")}
        </BarMenu>
        <BarMenu id="special" label="Special" {...barMenuProps}>
          {renderEntries("special")}
        </BarMenu>
        <BarMenu id="help" label="Help" {...barMenuProps}>
          {renderEntries("help")}
        </BarMenu>
      </div>

      <div className="mac-menu-bar-right">
        <button
          type="button"
          className="mac-menu-trigger"
          aria-label="切换到 Windows 98"
          title="切换到 Windows 98"
          onClick={() => {
            setOpenMenu(null);
            setOsTheme("win");
          }}
        >
          <Logo variant="16x16_4" width={16} height={16} aria-hidden />
        </button>
        <span className="mac-menu-clock">{clock}</span>
        <div className="mac-menu">
          <button
            type="button"
            className="mac-menu-trigger mac-window-menu-trigger"
            aria-label="应用程序菜单"
            aria-expanded={openMenu === "windows"}
            aria-controls="mac-window-menu"
            onClick={() => toggleMenu("windows")}
            onMouseEnter={() => trackMenu("windows")}
          >
            <MacIconFinder aria-hidden />
          </button>
          {openMenu === "windows" && (
            <ul id="mac-window-menu" className="mac-menu-dropdown mac-window-menu">
              {desktop.windows.length === 0 && (
                <li className="mac-menu-item mac-menu-item-disabled">没有打开的窗口</li>
              )}
              {desktop.windows.map((item) => (
                <li key={item.id}>
                  <button
                    type="button"
                    className={cn(
                      "mac-menu-item",
                      desktop.activeWindow === item.id && !item.minimized && "is-active",
                    )}
                    onClick={() => {
                      setOpenMenu(null);
                      openWindow(item.id);
                    }}
                  >
                    <span className="mac-menu-check" aria-hidden>
                      {desktop.activeWindow === item.id && !item.minimized ? "◆" : ""}
                    </span>
                    <span className="mac-window-menu-icon" aria-hidden>
                      {item.icon}
                    </span>
                    <span className="truncate">{item.title}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </header>
  );
}

/** One pull-down menu on the bar: a trigger plus its dropdown while open. */
function BarMenu({
  id,
  label,
  openMenu,
  toggleMenu,
  trackMenu,
  children,
}: {
  id: Exclude<MenuId, "windows">;
  label: ReactNode;
  openMenu: MenuId | null;
  toggleMenu: (menu: MenuId) => void;
  trackMenu: (menu: MenuId) => void;
  children?: ReactNode;
}) {
  return (
    <div className="mac-menu">
      <button
        type="button"
        className="mac-menu-trigger"
        aria-expanded={openMenu === id}
        aria-controls={`mac-${id}-menu`}
        onClick={() => toggleMenu(id)}
        onMouseEnter={() => trackMenu(id)}
      >
        {label}
      </button>
      {openMenu === id && (
        <ul id={`mac-${id}-menu`} className="mac-menu-dropdown">
          {children}
        </ul>
      )}
    </div>
  );
}
