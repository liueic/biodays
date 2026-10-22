"use client";

import { useDraggable } from "@neodrag/react";
import { Button, Frame, TitleBar } from "@react95/core";
import { Globe } from "@react95/icons";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  type ReactElement,
  type ReactNode,
  type RefObject,
  useEffect,
  useRef,
  useState,
} from "react";

const MENU = [
  { href: "/", label: "今日", key: "T" },
  { href: "/archive", label: "归档", key: "A" },
  { href: "/about", label: "关于", key: "B" },
];

// Remember where the user dropped the window so it stays put across page navigations.
let lastPosition = { x: 0, y: 0 };

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
  const windowRef = useRef<HTMLDivElement>(null) as RefObject<HTMLDivElement>;
  const [canDrag, setCanDrag] = useState(false);

  // Dragging is desktop-only; on narrow screens the window stays maximized.
  useEffect(() => {
    const query = window.matchMedia("(min-width: 768px)");
    const update = () => setCanDrag(query.matches);
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);

  useDraggable(windowRef, {
    handle: ".window-drag-handle",
    cancel: "button",
    bounds: "body",
    disabled: !canDrag,
    defaultPosition: lastPosition,
    onDragEnd: ({ offsetX, offsetY }) => {
      lastPosition = { x: offsetX, y: offsetY };
    },
  });

  return (
    <Frame
      ref={windowRef}
      aria-label={title}
      bgColor="$material"
      boxShadow="$out"
      padding="$2"
      className="mx-auto w-full max-w-[880px]"
    >
      <TitleBar active title={title} icon={icon} className="win98-title window-drag-handle">
        <TitleBar.OptionsBox>
          <TitleBar.Minimize aria-hidden tabIndex={-1} />
          <TitleBar.Maximize aria-hidden tabIndex={-1} />
          <TitleBar.Close aria-label="关闭，回到首页" onClick={() => router.push("/")} />
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
        className="min-h-[360px] sm:p-6"
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
