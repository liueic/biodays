"use client";

import { Button } from "@react95/core/Button";
import { Fieldset } from "@react95/core/Fieldset";
import { Input } from "@react95/core/Input";
import { type FormEvent, useState } from "react";

import { useAdminSession } from "@/components/admin/admin-session";
import { FeedbackApiError } from "@/lib/admin-api";

export function LoginForm() {
  const { login } = useAdminSession();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (!username.trim() || !password) {
      setError("请填写账号与密码。");
      return;
    }
    setSubmitting(true);
    try {
      await login(username.trim(), password);
      setPassword("");
    } catch (reason) {
      if (reason instanceof FeedbackApiError) {
        if (reason.status === 429) {
          setError("尝试次数过多，请稍后再试。");
        } else if (reason.status === 401) {
          setError("账号或密码不正确。");
        } else if (reason.status === 403) {
          setError("请求来源校验失败，请从本站登录页重试。");
        } else {
          setError(reason.message);
        }
      } else {
        setError("网络异常，请稍后重试。");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-4">
      <p className="reading">管理员使用部署配置的账号密码登录。访客提交反馈无需登录。</p>
      <Fieldset legend="管理员登录">
        <div className="flex flex-col gap-3">
          <div>
            <label htmlFor="admin-username" className="mb-1 block font-bold">
              账号
            </label>
            <Input
              id="admin-username"
              autoComplete="username"
              autoFocus
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              className="w-full"
            />
          </div>
          <div>
            <label htmlFor="admin-password" className="mb-1 block font-bold">
              密码
            </label>
            <Input
              id="admin-password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              className="w-full"
            />
          </div>
        </div>
      </Fieldset>
      {error && (
        <p
          role="alert"
          className="border border-[#a00000] bg-[#ffe8e8] px-3 py-2 text-[12px] text-[#7a0000]"
        >
          {error}
        </p>
      )}
      <div>
        <Button type="submit" className="px-6" disabled={submitting}>
          {submitting ? "登录中…" : "登录"}
        </Button>
      </div>
    </form>
  );
}
