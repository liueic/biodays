"use client";

import { Button } from "@react95/core/Button";
import { useEffect, useState } from "react";

import { confirmSubscription, FeedbackApiError, unsubscribe } from "@/lib/feedback-api";

type Mode = "confirm" | "unsubscribe";
type State = "reading" | "idle" | "working" | "done" | "error";

const CONFIRM_COPY: Record<string, string> = {
  confirmed: "订阅已确认。这条反馈有处理结果时，你会收到一封邮件。",
  already_confirmed: "这条反馈的订阅已经确认过了，无需重复操作。",
  unsubscribed: "这条反馈的通知已退订，确认链接不会重新激活订阅。",
};

const UNSUBSCRIBE_COPY: Record<string, string> = {
  unsubscribed: "已退订。这条反馈后续的状态变化不会再发送邮件。",
  already_unsubscribed: "这条反馈已经退订过了。",
};

function errorMessage(error: unknown): string {
  if (error instanceof FeedbackApiError) {
    if (error.code === "TOKEN_EXPIRED") {
      return "链接已过期。请回到反馈页重新发送确认邮件。";
    }
    if (error.code === "INVALID_TOKEN") {
      return "链接无效或已经使用过。请从邮件中的按钮重新进入。";
    }
    if (error.code === "FEEDBACK_NOT_FOUND") {
      return "对应的反馈已被清理，无法继续操作。";
    }
    if (error.code === "FEEDBACK_UNAVAILABLE") {
      return "反馈功能当前未启用，请稍后再试。";
    }
    return error.message;
  }
  return "网络异常，请稍后重试。";
}

export function TokenAction({ mode }: { mode: Mode }) {
  const [state, setState] = useState<State>("reading");
  const [token, setToken] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    const fragment = window.location.hash.startsWith("#") ? window.location.hash.slice(1) : "";
    const value = new URLSearchParams(fragment).get("token") ?? "";
    if (value) {
      setToken(value);
      setState("idle");
    } else {
      setState("error");
      setMessage("链接不完整。请从邮件中的按钮重新进入本页。");
    }
    // Keep the token out of the address bar and HTTP logs from here on.
    window.history.replaceState(null, "", window.location.pathname + window.location.search);
  }, []);

  async function handleAction() {
    if (state !== "idle" || !token) return;
    setState("working");
    setMessage("");
    try {
      const result =
        mode === "confirm" ? await confirmSubscription(token) : await unsubscribe(token);
      const copy =
        mode === "confirm" ? CONFIRM_COPY[result.result] : UNSUBSCRIBE_COPY[result.result];
      setMessage(copy ?? "操作已完成。");
      setState("done");
      setToken("");
    } catch (error) {
      setMessage(errorMessage(error));
      setState("error");
    }
  }

  if (state === "reading") {
    return (
      <p role="status" className="reading">
        正在读取链接…
      </p>
    );
  }

  if (state === "error") {
    return (
      <div className="reading">
        <p role="alert">{message}</p>
        <p className="mt-3">
          返回{" "}
          <a className="text-link" href="/feedback">
            灵感信箱
          </a>
          。
        </p>
      </div>
    );
  }

  if (state === "done") {
    return (
      <div className="reading">
        <p role="status">{message}</p>
        <p className="mt-3">
          返回{" "}
          <a className="text-link" href="/">
            今日故事
          </a>
          。
        </p>
      </div>
    );
  }

  return (
    <div className="reading">
      {mode === "confirm" ? (
        <>
          <p>确认后会订阅这条反馈的处理结果。确认前不会发送结果邮件。</p>
          <p className="mt-2 text-[12px] text-[#555]">
            页面只会在你点击按钮后提交，邮件扫描器直接打开链接不会改变订阅状态。
          </p>
        </>
      ) : (
        <p>退订后，这条反馈后续的状态变化不会再发送邮件。已交给邮件服务商的邮件无法撤回。</p>
      )}
      <div className="mt-4 flex flex-wrap gap-3">
        <Button
          type="button"
          className="px-5"
          onClick={handleAction}
          disabled={state === "working"}
        >
          {state === "working" ? "处理中…" : mode === "confirm" ? "确认订阅" : "确认退订"}
        </Button>
        <Button as="a" href="/feedback" className="px-5">
          返回信箱
        </Button>
      </div>
    </div>
  );
}
