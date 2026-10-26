"use client";

import { Button } from "@react95/core/Button";
import { FileText } from "@react95/icons/FileText";
import { useEffect } from "react";

import { ErrorState } from "@/components/error-state";
import { Window } from "@/components/win/window";

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
    <main>
      <Window
        title="页面出错了 - biodays"
        icon={<FileText variant="16x16_4" aria-hidden />}
        status={["错误"]}
      >
        <ErrorState
          title="页面出错了"
          code={error.digest}
          variant="critical"
          description="渲染时遇到了未预期的错误，可以重试，或回到今日故事。"
          action={{ href: "/", label: "回到今日" }}
        >
          <Button onClick={reset} className="px-6">
            重试
          </Button>
        </ErrorState>
      </Window>
    </main>
  );
}
