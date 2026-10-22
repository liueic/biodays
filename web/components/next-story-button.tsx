"use client";

import { useRouter } from "next/navigation";

import { RandomButton } from "@/components/random-button";

export function NextStoryButton({ currentId }: { currentId: string }) {
  const router = useRouter();

  return (
    <RandomButton
      exclude={[currentId]}
      label="随机一篇(N)"
      onStory={(story) => router.push(`/story/${story.id}`)}
    />
  );
}
