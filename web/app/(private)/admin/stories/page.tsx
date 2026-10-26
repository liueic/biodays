import { Bookmark } from "@react95/icons/Bookmark";
import type { Metadata } from "next";

import { StoryManager } from "@/components/admin/story-manager";
import { Window } from "@/components/win/window";

export const metadata: Metadata = {
  title: "故事管理",
};

export default function AdminStoriesPage() {
  return (
    <main>
      <Window
        title="故事管理 - biodays"
        icon={<Bookmark variant="16x16_4" aria-hidden />}
        status={["管理员", "biodays"]}
        variant="mail"
        width="wide"
      >
        <h1 className="sr-only">故事管理</h1>
        <StoryManager />
      </Window>
    </main>
  );
}
