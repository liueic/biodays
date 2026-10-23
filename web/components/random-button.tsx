"use client";

import { Button } from "@react95/core/Button";
import { useState } from "react";

import { getRandomStory, type Story } from "@/lib/api";

type RandomButtonProps = {
  exclude?: string[];
  onStory: (story: Story) => void;
  label?: string;
};

export function RandomButton({
  exclude = [],
  onStory,
  label = "下一条提示(N)",
}: RandomButtonProps) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function draw() {
    setPending(true);
    setError(null);
    try {
      onStory(await getRandomStory(exclude));
    } catch {
      setError("获取失败，请稍后再试。");
    } finally {
      setPending(false);
    }
  }

  return (
    <>
      <Button onClick={draw} disabled={pending} className="px-4">
        {pending ? "请稍候…" : label}
      </Button>
      <span aria-live="polite" className="text-[#b00000]">
        {error}
      </span>
    </>
  );
}
