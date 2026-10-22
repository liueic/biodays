"use client";

import { Button } from "@react95/core";
import { useEffect } from "react";
import { ErrorState } from "@/components/error-state";

export default function ErrorPage({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <main className="mx-auto max-w-3xl px-5">
      <ErrorState
        title="页面出错了"
        description="渲染时遇到了未预期的错误，可以重试，或回到首页。"
        action={{ href: "/", label: "回到今日" }}
      >
        <Button onClick={reset} className="px-6">
          重试
        </Button>
      </ErrorState>
    </main>
  );
}
