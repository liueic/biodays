"use client";

import { Button } from "@react95/core/Button";
import { Checkbox } from "@react95/core/Checkbox";
import { Dropdown } from "@react95/core/Dropdown";
import { Input } from "@react95/core/Input";
import { TextArea } from "@react95/core/TextArea";
import { Mail } from "@react95/icons/Mail";
import { Sendmail2001 } from "@react95/icons/Sendmail2001";
import { useSearchParams } from "next/navigation";
import { type FormEvent, useEffect, useMemo, useState } from "react";

import {
  FeedbackApiError,
  type FeedbackKind,
  type FeedbackOptions,
  type FeedbackReceipt,
  fetchFeedbackOptions,
  newRequestID,
  resendConfirmation,
  submitFeedback,
} from "@/lib/feedback-api";

type FieldState = {
  title: string;
  content: string;
  sourceURL: string;
  nickname: string;
  email: string;
  subscribe: boolean;
};

const EMPTY_FIELDS: FieldState = {
  title: "",
  content: "",
  sourceURL: "",
  nickname: "",
  email: "",
  subscribe: false,
};

const KIND_ORDER: FeedbackKind[] = ["topic", "resource", "feature", "issue"];

function runeLength(value: string): number {
  return Array.from(value).length;
}

function isEmailShaped(value: string): boolean {
  return /^[^\s@<>]+@[^\s@<>]+$/.test(value.trim());
}

export function FeedbackForm() {
  const params = useSearchParams();
  const initialKind = useMemo<FeedbackKind>(() => {
    const value = params.get("kind");
    return KIND_ORDER.includes(value as FeedbackKind) ? (value as FeedbackKind) : "topic";
  }, [params]);
  const storyID = useMemo(() => (params.get("story_id") ?? "").slice(0, 64), [params]);

  const [options, setOptions] = useState<FeedbackOptions | null>(null);
  const [optionsError, setOptionsError] = useState("");
  const [kind, setKind] = useState<FeedbackKind>(initialKind);
  const [fields, setFields] = useState<FieldState>(EMPTY_FIELDS);
  const [website, setWebsite] = useState("");
  const [requestID, setRequestID] = useState(() => newRequestID());
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [generalError, setGeneralError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [receipt, setReceipt] = useState<FeedbackReceipt | null>(null);
  const [resendState, setResendState] = useState<"idle" | "sending" | "sent">("idle");

  useEffect(() => {
    let cancelled = false;
    fetchFeedbackOptions()
      .then((data) => {
        if (cancelled) return;
        setOptions(data);
        if (!data.enabled) return;
        setKind((current) =>
          data.kinds.some((item) => item.value === current)
            ? current
            : (data.kinds[0]?.value ?? "topic"),
        );
      })
      .catch(() => {
        if (!cancelled) setOptionsError("暂时无法读取反馈配置，请稍后刷新重试。");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function update<K extends keyof FieldState>(key: K, value: FieldState[K]) {
    setFields((current) => ({ ...current, [key]: value }));
    setErrors((current) => {
      if (!current[key]) return current;
      const next = { ...current };
      delete next[key];
      return next;
    });
  }

  function validate(): boolean {
    const next: Record<string, string> = {};
    if (!kind) next.kind = "请选择反馈类型";
    if (runeLength(fields.title.trim()) === 0) next.title = "请填写标题";
    if (runeLength(fields.content.trim()) < 10) next.content = "内容至少 10 个字符";
    if (fields.subscribe && !fields.email.trim()) next.email = "勾选邮件通知后需要填写邮箱";
    else if (fields.email.trim() && !isEmailShaped(fields.email)) next.email = "邮箱格式不正确";
    if (fields.sourceURL.trim() && !/^https?:\/\//i.test(fields.sourceURL.trim())) {
      next.source_url = "链接需要以 http:// 或 https:// 开头";
    }
    setErrors(next);
    const firstInvalidField = ["kind", "title", "content", "email", "source_url"].find(
      (field) => next[field],
    );
    if (firstInvalidField) {
      const fieldIDs: Record<string, string> = {
        kind: "feedback-kind",
        title: "feedback-title",
        content: "feedback-content",
        email: "feedback-email",
        source_url: "feedback-source",
      };
      requestAnimationFrame(() => document.getElementById(fieldIDs[firstInvalidField])?.focus());
    }
    return Object.keys(next).length === 0;
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setGeneralError("");
    if (!validate()) return;
    setSubmitting(true);
    try {
      const result = await submitFeedback({
        request_id: requestID,
        kind,
        title: fields.title.trim(),
        content: fields.content.trim(),
        source_url: fields.sourceURL.trim() || undefined,
        story_id: storyID || undefined,
        nickname: fields.nickname.trim() || undefined,
        email: fields.email.trim() || undefined,
        subscribe_updates: fields.subscribe,
        website,
      });
      setReceipt(result);
      setResendState("idle");
    } catch (error) {
      if (error instanceof FeedbackApiError) {
        if (Object.keys(error.fields).length > 0) setErrors(error.fields);
        setGeneralError(error.message);
      } else {
        setGeneralError("网络异常，反馈未提交。请保留内容后重试。");
      }
    } finally {
      setSubmitting(false);
    }
  }

  async function handleResend() {
    if (!receipt || resendState !== "idle") return;
    setResendState("sending");
    try {
      await resendConfirmation(receipt.id, fields.email.trim());
      setResendState("sent");
    } catch {
      setResendState("sent");
    }
  }

  function writeAnother() {
    setReceipt(null);
    setFields(EMPTY_FIELDS);
    setWebsite("");
    setErrors({});
    setGeneralError("");
    setRequestID(newRequestID());
    setResendState("idle");
  }

  if (optionsError) {
    return (
      <div className="mail-receipt">
        <p role="alert" className="mail-alert">
          {optionsError}
        </p>
      </div>
    );
  }

  if (!options) {
    return (
      <div className="mail-preview-empty" role="status">
        <Mail variant="32x32_4" aria-hidden />
        <p>正在打开新邮件…</p>
      </div>
    );
  }

  if (!options.enabled) {
    return (
      <div className="mail-receipt">
        <p className="mail-receipt-copy">
          灵感信箱暂时未开放。你可以先继续阅读故事，稍后再来看看。
        </p>
      </div>
    );
  }

  if (receipt) {
    return (
      <Receipt
        receipt={receipt}
        email={fields.email}
        resendState={resendState}
        onResend={handleResend}
        onAgain={writeAnother}
      />
    );
  }

  const limits = options.limits;
  const titleCount = runeLength(fields.title);
  const contentCount = runeLength(fields.content);
  const kindOptions = options.kinds.map((item) => item.label);
  const kindLabel =
    options.kinds.find((item) => item.value === kind)?.label ?? kindOptions[0] ?? "";

  return (
    <form onSubmit={handleSubmit} noValidate className="mail-compose" aria-label="写邮件到灵感信箱">
      <div className="mail-toolbar" role="toolbar" aria-label="邮件操作">
        <div className="mail-toolbar-group">
          <Button type="submit" disabled={submitting} className="mail-tool-button">
            <Sendmail2001 variant="32x32_4" aria-hidden />
            <span>{submitting ? "发送中…" : "发送(S)"}</span>
          </Button>
        </div>
        <div className="mail-toolbar-separator" aria-hidden="true" />
        <div className="mail-toolbar-group">
          <Button
            type="button"
            className="mail-tool-button mail-tool-button--inline"
            onClick={() => document.getElementById("feedback-content")?.focus()}
          >
            写正文
          </Button>
        </div>
      </div>

      {generalError && (
        <p role="alert" className="mail-alert">
          {generalError} 输入内容仍保留在本页，可直接重试。
        </p>
      )}

      <div className="mail-compose-fields">
        <div className="mail-compose-row">
          <span className="mail-compose-label">收件人</span>
          <div className="mail-compose-control">
            <div className="mail-compose-recipient">
              <Mail variant="16x16_4" width={16} height={16} aria-hidden />
              <span>biodays 维护者</span>
              <span className="mail-compose-hint">&lt;灵感信箱&gt;</span>
            </div>
          </div>
        </div>

        <div className="mail-compose-row">
          <label className="mail-compose-label" htmlFor="feedback-nickname">
            发件人
          </label>
          <div className="mail-compose-control mail-compose-sender">
            <div className="mail-compose-sender-field">
              <Input
                id="feedback-nickname"
                value={fields.nickname}
                maxLength={limits.nickname_max * 2}
                onChange={(event) => update("nickname", event.target.value)}
                aria-invalid={errors.nickname ? true : undefined}
                aria-label="称呼，留空为匿名访客"
                autoComplete="nickname"
                placeholder="称呼（留空为匿名访客）"
              />
              {errors.nickname && <FieldError>{errors.nickname}</FieldError>}
            </div>
            <div className="mail-compose-sender-field">
              <Input
                id="feedback-email"
                type="email"
                inputMode="email"
                autoComplete="email"
                value={fields.email}
                onChange={(event) => update("email", event.target.value)}
                aria-invalid={errors.email ? true : undefined}
                aria-label="联系邮箱，选填"
                placeholder="联系邮箱（选填）"
              />
              {errors.email && <FieldError>{errors.email}</FieldError>}
            </div>
          </div>
        </div>

        <div className="mail-compose-row">
          <label className="mail-compose-label" htmlFor="feedback-title">
            主题
          </label>
          <div className="mail-compose-control mail-compose-subject">
            <div className="mail-compose-subject-kind">
              <Dropdown
                id="feedback-kind"
                options={kindOptions}
                value={kindLabel}
                aria-label="反馈类型"
                aria-invalid={errors.kind ? true : undefined}
                onChange={(event) => {
                  const selected = options.kinds.find((item) => item.label === event.target.value);
                  if (selected) setKind(selected.value);
                }}
              />
              {errors.kind && <FieldError>{errors.kind}</FieldError>}
            </div>
            <div className="mail-compose-subject-title">
              <Input
                id="feedback-title"
                value={fields.title}
                maxLength={limits.title_max * 2}
                onChange={(event) => update("title", event.target.value)}
                aria-invalid={errors.title ? true : undefined}
                aria-describedby="feedback-title-count"
                placeholder="用一句话概括这封邮件"
              />
              <div id="feedback-title-count" className="mail-compose-field-meta">
                <span>邮件主题</span>
                <span>
                  {titleCount}/{limits.title_max}
                </span>
              </div>
              {errors.title && <FieldError>{errors.title}</FieldError>}
            </div>
          </div>
        </div>
      </div>

      <div className="mail-compose-body">
        <div className="mail-compose-body-heading">
          <label htmlFor="feedback-content">正文</label>
          <span>纯文本</span>
        </div>
        <TextArea
          id="feedback-content"
          rows={10}
          value={fields.content}
          onChange={(event) => update("content", event.target.value)}
          aria-invalid={errors.content ? true : undefined}
          aria-describedby="feedback-content-count"
          className="mail-compose-body-input"
          placeholder="请写下想了解的话题、推荐的资源或需要核对的问题…"
        />
        {errors.content && (
          <div className="px-2 pb-1">
            <FieldError>{errors.content}</FieldError>
          </div>
        )}
        <div id="feedback-content-count" className="mail-compose-body-footer">
          <span>
            至少 {limits.content_min} 个字符，最多 {limits.content_max} 个字符
          </span>
          <span>
            {contentCount}/{limits.content_max}
          </span>
        </div>
      </div>

      <details className="mail-compose-options">
        <summary>邮件选项（相关链接、通知与隐私）</summary>
        <div className="mail-compose-options-content">
          {storyID && (
            <div className="mail-compose-story">
              关联故事：<span className="font-mono">{storyID}</span>
            </div>
          )}

          <div>
            <label htmlFor="feedback-source" className="mail-compose-option-label">
              相关链接（选填）
            </label>
            <Input
              id="feedback-source"
              type="url"
              inputMode="url"
              placeholder="https://…"
              value={fields.sourceURL}
              onChange={(event) => update("sourceURL", event.target.value)}
              aria-invalid={errors.source_url ? true : undefined}
            />
            {errors.source_url && <FieldError>{errors.source_url}</FieldError>}
          </div>

          <div>
            <Checkbox
              id="feedback-subscribe"
              checked={fields.subscribe}
              disabled={!options.subscriptions_enabled}
              onChange={(event) => update("subscribe", event.target.checked)}
              label="通过邮件接收处理结果"
            />
            <p className="mail-compose-hint">
              {options.subscriptions_enabled
                ? "勾选后需要到邮箱确认订阅；邮箱仅用于本条反馈的联系与通知。"
                : "邮件通知当前不可用，仍可正常发送反馈。"}
            </p>
            {errors.subscribe_updates && <FieldError>{errors.subscribe_updates}</FieldError>}
          </div>
        </div>
      </details>

      <div
        aria-hidden="true"
        style={{
          position: "absolute",
          left: "-10000px",
          top: "auto",
          width: 1,
          height: 1,
          overflow: "hidden",
        }}
      >
        <label htmlFor="feedback-website">网站（请留空）</label>
        <input
          id="feedback-website"
          name="website"
          tabIndex={-1}
          autoComplete="off"
          value={website}
          onChange={(event) => setWebsite(event.target.value)}
        />
      </div>

      <div className="mail-compose-status">
        <span className="mail-status-cell">无需注册，可匿名发送；反馈不会公开展示。</span>
        <span className="mail-status-cell mail-status-cell--shrink">
          {titleCount > 0 || contentCount > 0 ? "正在编辑" : "新邮件"}
        </span>
      </div>
    </form>
  );
}

function FieldError({ children }: { children: React.ReactNode }) {
  return (
    <p role="alert" className="mail-field-error">
      {children}
    </p>
  );
}

function Receipt({
  receipt,
  email,
  resendState,
  onResend,
  onAgain,
}: {
  receipt: FeedbackReceipt;
  email: string;
  resendState: "idle" | "sending" | "sent";
  onResend: () => void;
  onAgain: () => void;
}) {
  const pending = receipt.subscription_status === "pending_confirmation";
  return (
    <div className="mail-receipt">
      <div className="mail-receipt-heading">
        <Sendmail2001 variant="32x32_4" aria-hidden />
        <div>
          <h2>邮件已投递到灵感信箱</h2>
          <p className="mail-compose-hint">反馈已保存，维护者会在收件箱中处理。</p>
        </div>
      </div>

      <p role="status" className="mail-receipt-copy">
        邮件编号：<span className="font-mono font-bold">{receipt.id}</span>
      </p>

      {pending ? (
        <div className="mail-receipt-notice mail-receipt-copy">
          <p>
            请到 {email || "你填写的邮箱"}{" "}
            确认订阅，确认前不会发送处理结果。若没有收到，可以重新发送确认邮件。
          </p>
          <div className="mail-receipt-actions mt-3">
            <Button
              type="button"
              className="mail-tool-button mail-tool-button--inline"
              onClick={onResend}
              disabled={resendState !== "idle"}
            >
              {resendState === "sending"
                ? "发送中…"
                : resendState === "sent"
                  ? "已请求发送"
                  : "重发确认邮件"}
            </Button>
            {resendState === "sent" && (
              <span role="status">如果邮箱与这条反馈匹配，确认邮件会很快到达。</span>
            )}
          </div>
        </div>
      ) : (
        <p className="mail-receipt-copy">没有订阅通知。管理员会在后台处理这条反馈。</p>
      )}

      <div className="mail-receipt-actions">
        <Button
          type="button"
          className="mail-tool-button mail-tool-button--inline"
          onClick={onAgain}
        >
          再写一封
        </Button>
      </div>
    </div>
  );
}
