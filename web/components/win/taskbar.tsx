"use client";

import { Frame } from "@react95/core/Frame";
import { Computer } from "@react95/icons/Computer";
import { FileFind } from "@react95/icons/FileFind";
import { FileText } from "@react95/icons/FileText";
import { Folder } from "@react95/icons/Folder";
import { HelpBook } from "@react95/icons/HelpBook";
import { Logo } from "@react95/icons/Logo";
import { Mail } from "@react95/icons/Mail";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { MacIconHappyMac } from "@/components/mac/icons";
import { MAIN_WINDOW, openWindow, toggleWindow, useWindows } from "@/components/win/window-store";
import { setOsTheme } from "@/hooks/use-os-theme";
import { DOCS_URL } from "@/lib/site";

export function DesktopTaskbar() {
  const router = useRouter();

  const desktop = useWindows();
  const [open, setOpen] = useState(false);
  const [clock, setClock] = useState("--:--");
  const ref = useRef<HTMLElement>(null);

  useEffect(() => {
    const update = () =>
      setClock(
        new Date().toLocaleTimeString("zh-CN", {
          hour: "2-digit",
          minute: "2-digit",
          hour12: false,
        }),
      );
    update();
    const timer = window.setInterval(update, 30_000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!open) return;
    const dismiss = (event: PointerEvent) => {
      if (!ref.current?.contains(event.target as Node)) setOpen(false);
    };
    const onEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", dismiss);
    document.addEventListener("keydown", onEscape);
    return () => {
      document.removeEventListener("pointerdown", dismiss);
      document.removeEventListener("keydown", onEscape);
    };
  }, [open]);

  function navigate(path: string) {
    setOpen(false);
    openWindow(MAIN_WINDOW);
    // Cross-group target: a full page load unloads the reading layout's
    // session recorder before the private pages render.
    if (path === "/feedback") {
      window.location.assign(path);
      return;
    }
    router.push(path);
  }

  return (
    <nav ref={ref} className="desktop-taskbar" aria-label="桌面任务栏">
      {open && (
        <Frame
          id="desktop-start-menu"
          boxShadow="$out"
          bgColor="$material"
          className="absolute bottom-7 left-0"
        >
          <ul className="min-w-40 py-1">
            {[
              { label: "今日故事", path: "/", icon: <FileText variant="32x32_4" /> },
              { label: "归档", path: "/archive", icon: <Folder variant="32x32_4" /> },
              { label: "实验室手册", path: "/lab", icon: <HelpBook variant="32x32_4" /> },
              { label: "关于", path: "/about", icon: <FileFind variant="32x32_4" /> },
              { label: "灵感信箱", path: "/feedback", icon: <Mail variant="32x32_4" /> },
            ].map((item) => (
              <li key={item.path}>
                <button
                  type="button"
                  className="menu-item flex w-full items-center gap-2 text-left"
                  onClick={() => navigate(item.path)}
                >
                  {item.icon}
                  {item.label}
                </button>
              </li>
            ))}
            <li className="start-menu-divider mt-1 pt-1">
              <button
                type="button"
                className="menu-item flex w-full items-center gap-2 text-left"
                onClick={() => {
                  setOpen(false);
                  window.open(DOCS_URL, "_blank", "noopener");
                }}
              >
                <HelpBook variant="32x32_4" />
                API 文档
              </button>
            </li>
            <li>
              <button
                type="button"
                className="menu-item flex w-full items-center gap-2 text-left"
                onClick={() => {
                  setOpen(false);
                  setOsTheme("mac");
                }}
              >
                <MacIconHappyMac aria-hidden />
                切换到 Mac
              </button>
            </li>
            <li>
              <button
                type="button"
                className="menu-item flex w-full items-center gap-2 text-left"
                onClick={() => window.location.reload()}
              >
                <Computer variant="32x32_4" />
                重置桌面布局
              </button>
            </li>
          </ul>
        </Frame>
      )}
      <button
        type="button"
        className="taskbar-button shrink-0 font-bold"
        aria-expanded={open}
        aria-controls="desktop-start-menu"
        onClick={() => setOpen(!open)}
      >
        <Logo variant="16x16_4" aria-hidden /> 开始
      </button>
      <div className="flex min-w-0 flex-1 gap-1">
        {desktop.windows.map((item) => (
          <button
            key={item.id}
            type="button"
            className="taskbar-button max-w-48 flex-1"
            title={item.title}
            aria-pressed={desktop.activeWindow === item.id && !item.minimized}
            onClick={() => toggleWindow(item.id)}
          >
            <span className="taskbar-icon shrink-0">{item.icon}</span>
            <span className="truncate">{item.title}</span>
          </button>
        ))}
      </div>
      <button
        type="button"
        className="taskbar-button shrink-0"
        aria-label="切换到 Mac"
        title="切换到 Mac"
        onClick={() => {
          setOpen(false);
          setOsTheme("mac");
        }}
      >
        <MacIconHappyMac width={16} height={16} aria-hidden />
      </button>
      <Frame boxShadow="$in" className="shrink-0 px-3 py-1" aria-label="当前时间">
        {clock}
      </Frame>
    </nav>
  );
}
