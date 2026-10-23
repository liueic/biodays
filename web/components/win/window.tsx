"use client";

import { Button, Frame, TitleBar } from "@react95/core";
import { Globe } from "@react95/icons";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import type { ReactElement, ReactNode } from "react";
import { useWindowFrame } from "@/components/win/use-window-frame";
import { MAIN_WINDOW } from "@/components/win/window-store";
import { cn } from "@/lib/utils";

const MENU = [
  { href: "/", label: "今日", key: "T" },
  { href: "/archive", label: "归档", key: "A" },
  { href: "/about", label: "关于", key: "B" },
];

type WindowProps = {
  title: string;
  icon: ReactElement;
  status?: ReactNode[];
  children: ReactNode;
};

/** A Win98 application window: title bar, menu bar, toolbar, sunken client area, status bar. */
export function Window({ title, icon, status = [], children }: WindowProps) {
  const pathname = usePathname();
  const router = useRouter();
  const frame = useWindowFrame<HTMLDivElement>({
    id: MAIN_WINDOW,
    title,
    icon,
    reopenOnMount: true,
  });

  return (
    <Frame
      ref={frame.ref}
      {...frame.focusProps}
      aria-label={title}
      bgColor="$material"
      boxShadow="$out"
      padding="$2"
      className={cn(
        "relative mx-auto w-full max-w-[880px]",
        frame.hidden && "hidden",
        frame.maximized && !frame.hidden && "win-maximized",
      )}
    >
      <TitleBar
        active={frame.active}
        title={title}
        icon={icon}
        className="win98-title window-drag-handle"
        data-inactive={frame.active ? undefined : ""}
        onDoubleClick={frame.toggleMaximize}
      >
        <TitleBar.OptionsBox>
          <TitleBar.Minimize aria-label="最小化" onClick={frame.minimize} />
          {frame.maximized ? (
            <TitleBar.Restore aria-label="还原" onClick={frame.toggleMaximize} />
          ) : (
            <TitleBar.Maximize aria-label="最大化" onClick={frame.toggleMaximize} />
          )}
          <TitleBar.Close aria-label="关闭" onClick={frame.close} />
        </TitleBar.OptionsBox>
      </TitleBar>

      <nav aria-label="主菜单" className="flex gap-1 px-1 py-1">
        {MENU.map((item) => {
          const active = item.href === "/" ? pathname === "/" : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              className="menu-item"
              aria-current={active ? "page" : undefined}
            >
              {item.label}({item.key})
            </Link>
          );
        })}
      </nav>

      <Frame
        display="flex"
        alignItems="center"
        gap="$6"
        padding="$4"
        className="border-t border-[#868a8e]"
      >
        <Button onClick={() => router.back()}>后退</Button>
        <Button onClick={() => router.push("/")}>主页</Button>
        <span className="ml-2">地址(D)</span>
        <Frame
          boxShadow="$in"
          bgColor="$inputBackground"
          display="flex"
          alignItems="center"
          gap="$4"
          padding="$4"
          className="min-w-0 flex-1"
        >
          <Globe variant="16x16_4" width={16} height={16} aria-hidden />
          <span className="truncate">biodays:/{pathname}</span>
        </Frame>
      </Frame>

      <Frame
        as="div"
        id="window-content"
        boxShadow="$in"
        bgColor="$inputBackground"
        padding="$16"
        className="window-content min-h-[360px] sm:p-6"
      >
        {children}
      </Frame>

      <div className="mt-[2px] flex gap-[2px]">
        {status.map((cell, index) => (
          <Frame
            // biome-ignore lint/suspicious/noArrayIndexKey: static status cells
            key={index}
            boxShadow="$in"
            padding="$2"
            className="truncate px-2 first:flex-1"
          >
            {cell}
          </Frame>
        ))}
      </div>
    </Frame>
  );
}
