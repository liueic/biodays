"use client";

import { Button } from "@react95/core/Button";
import { Mail } from "@react95/icons/Mail";
import { useEffect } from "react";

import { ErrorState } from "@/components/error-state";
import { Window } from "@/components/win/window";

export default function PrivateError({
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
        icon={<Mail variant="16x16_4" aria-hidden />}
        status={["错误"]}
      >
        <ErrorState
          title="页面出错了"
          code={error.digest}
          variant="critical"
          description="可以重试，或返回灵感信箱。输入内容不会被自动保存。"
          action={{ href: "/feedback", label: "返回灵感信箱" }}
        >
          <Button onClick={reset} className="px-6">
            重试
          </Button>
        </ErrorState>
      </Window>
    </main>
  );
}
