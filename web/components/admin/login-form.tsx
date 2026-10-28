"use client";

import { Button } from "@react95/core/Button";
import { useState } from "react";
import { FeedbackApiError, githubLogin } from "@/lib/admin-api";

export function LoginForm({ initialError = "" }: { initialError?: string }) {
  const [error, setError] = useState(initialError);
  const [submitting, setSubmitting] = useState(false);

  async function handleLogin() {
    setError("");
    setSubmitting(true);
    try {
      window.location.assign(await githubLogin());
    } catch (reason) {
      setError(
        reason instanceof FeedbackApiError ? reason.message : "无法连接 GitHub 登录，请稍后重试。",
      );
      setSubmitting(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="reading">仅允许白名单中的 GitHub 账号进入后台。访客提交反馈无需登录。</p>
      <div>
        <Button type="button" onClick={handleLogin} disabled={submitting} className="px-6">
          {submitting ? "正在前往 GitHub…" : "使用 GitHub 账号登录"}
        </Button>
      </div>
      {error && (
        <p
          role="alert"
          className="border border-[#a00000] bg-[#ffe8e8] px-3 py-2 text-[12px] text-[#7a0000]"
        >
          {error}
        </p>
      )}
    </div>
  );
}
