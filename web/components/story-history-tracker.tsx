"use client";

import { useEffect } from "react";
import { recordStoryView } from "@/lib/history";

export function StoryHistoryTracker({ storyId }: { storyId: string }) {
  useEffect(() => {
    void recordStoryView(storyId);
  }, [storyId]);
  return null;
}
