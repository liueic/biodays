"use client";

import { useRouter } from "next/navigation";
import { useRef, useState } from "react";

import { MAIN_WINDOW, openWindow } from "@/components/win/window-store";
import { getRandomStory } from "@/lib/api";

/** The auxiliary random command is available in either system menu. */
export function useRandomStory(onOpen: () => void) {
  const router = useRouter();
  const pending = useRef(false);
  const [state, setState] = useState<"idle" | "pending" | "error">("idle");

  async function openRandom() {
    if (pending.current) return;
    pending.current = true;
    setState("pending");
    try {
      const story = await getRandomStory();
      openWindow(MAIN_WINDOW);
      router.push(`/story/${story.id}`);
      onOpen();
      setState("idle");
    } catch {
      setState("error");
    } finally {
      pending.current = false;
    }
  }

  return {
    openRandom,
    pending: state === "pending",
    label: state === "pending" ? "请稍候…" : state === "error" ? "获取失败，点击重试" : "随机一篇",
  };
}
