"use client";

import { Button } from "@react95/core/Button";
import { Frame } from "@react95/core/Frame";
import { TitleBar } from "@react95/core/TitleBar";
import { Globe } from "@react95/icons/Globe";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import type { ReactElement, ReactNode } from "react";
import { MacIconGlobe } from "@/components/mac/icons";
import { MacTitleBar } from "@/components/mac/window";
import { useWindowFrame } from "@/components/win/use-window-frame";
import { MAIN_WINDOW } from "@/components/win/window-store";
import { cn } from "@/lib/utils";

const MENU = [
  { href: "/", label: "今日", key: "T" },
  { href: "/archive", label: "归档", key: "A" },
  { href: "/lab", label: "手册", key: "L" },
  { href: "/about", label: "关于", key: "B" },
  // external marks links into the (private) group: they must be full page
  // loads so the session recorder loaded in the reading layout unloads.
  { href: "/feedback", label: "反馈", key: "F", external: true },
];

type WindowProps = {
  title: string;
  icon: ReactElement;
  status?: ReactNode[];
  children: ReactNode;
  variant?: "browser" | "mail";
  width?: "default" | "wide";
};

/** Shared window: React95 controls and the reference Platinum skin share one content tree. */
export function Window({
  title,
  icon,
  status = [],
  children,
  variant = "browser",
  width = "default",
}: WindowProps) {
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
      data-active={frame.active || undefined}
      data-shaded={frame.shaded || undefined}
      bgColor="$material"
      boxShadow="$out"
      padding="$2"
      className={cn(
        "os-window mac-window relative mx-auto w-full min-w-0",
        width === "wide" ? "max-w-[1220px]" : "max-w-[880px]",
        frame.hidden && "hidden",
        frame.maximized && !frame.hidden && "os-maximized mac-maximized",
      )}
    >
      <TitleBar
        active={frame.active}
        title={title}
        icon={icon}
        className="win98-title window-drag-handle"
        data-os-only="win"
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
      <MacTitleBar
        title={title}
        shaded={frame.shaded}
        maximized={frame.maximized}
        onClose={frame.close}
        onZoom={frame.toggleMaximize}
        onToggleShade={frame.toggleShade}
      />

      <div className="os-window-body">
        <nav aria-label="主菜单" data-os-only="win" className="flex gap-1 px-1 py-1">
          {MENU.map((item) => {
            const active = item.href === "/" ? pathname === "/" : pathname.startsWith(item.href);
            const className = "menu-item";
            const content = (
              <>
                {item.label}({item.key})
              </>
            );
            if (item.external) {
              return (
                <a key={item.href} href={item.href} className={className}>
                  {content}
                </a>
              );
            }
            return (
              <Link
                key={item.href}
                href={item.href}
                className={className}
                aria-current={active ? "page" : undefined}
              >
                {content}
              </Link>
            );
          })}
        </nav>

        {variant === "browser" && (
          <Frame
            display="flex"
            alignItems="center"
            gap="$6"
            padding="$4"
            className="mac-toolbar flex-wrap border-t border-[#868a8e]"
          >
            <Button className="mac-button shrink-0 px-3 sm:px-5" onClick={() => router.back()}>
              后退
            </Button>
            <Button className="mac-button shrink-0 px-3 sm:px-5" onClick={() => router.push("/")}>
              主页
            </Button>
            <span className="ml-2 hidden shrink-0 sm:inline">地址(D)</span>
            <Frame
              boxShadow="$in"
              bgColor="$inputBackground"
              display="flex"
              alignItems="center"
              gap="$4"
              padding="$4"
              className="mac-address min-w-0 flex-[1_1_120px]"
            >
              <span data-os-only="win">
                <Globe variant="16x16_4" width={16} height={16} aria-hidden className="shrink-0" />
              </span>
              <span data-os-only="mac">
                <MacIconGlobe aria-hidden className="shrink-0" />
              </span>
              <span className="truncate">biodays:/{pathname}</span>
            </Frame>
          </Frame>
        )}

        <Frame
          as="div"
          id="window-content"
          boxShadow="$in"
          bgColor={variant === "mail" ? "$material" : "$inputBackground"}
          padding="$16"
          className={cn(
            "mac-window-content window-content min-h-[360px]",
            variant === "browser" ? "sm:p-6" : "mail-window-content p-0",
          )}
        >
          {children}
        </Frame>

        <div className="mac-window-status mt-[2px] flex min-w-0 gap-[2px]">
          {status.map((cell, index) => (
            <Frame
              // biome-ignore lint/suspicious/noArrayIndexKey: static status cells
              key={index}
              boxShadow="$in"
              padding="$2"
              className="mac-window-status-cell min-w-0 truncate px-2 first:flex-1"
            >
              {cell}
            </Frame>
          ))}
        </div>
      </div>
    </Frame>
  );
}
