"use client";

import { Button } from "@react95/core/Button";
import { Checkbox } from "@react95/core/Checkbox";
import { Dropdown } from "@react95/core/Dropdown";
import { Frame } from "@react95/core/Frame";
import { Input } from "@react95/core/Input";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { useAdminSession } from "@/components/admin/admin-session";
import {
  type AISpamPatch,
  FeedbackApiError,
  getSettings,
  listSettingsAudit,
  type RateLimitSettings,
  type RuntimeSettings,
  type SettingsAuditRecord,
  type SMTPPatch,
  type SpamSettings,
  type TurnstilePatch,
  updateSettings,
} from "@/lib/admin-api";
import { formatDateTime } from "@/lib/feedback-labels";

type Draft = {
  subscriptions: boolean;
  smtp: SMTPPatch & { password_input: string };
  rateLimits: RateLimitSettings;
  turnstile: TurnstilePatch & { secret_input: string; hostnames_input: string };
  spam: SpamSettings;
  aiSpam: AISpamPatch & { token_input: string };
};

function draftFrom(settings: RuntimeSettings): Draft {
  return {
    subscriptions: settings.subscriptions_enabled,
    smtp: {
      enabled: settings.smtp.enabled,
      host: settings.smtp.host,
      port: settings.smtp.port,
      tls_mode: settings.smtp.tls_mode,
      username: settings.smtp.username,
      from: settings.smtp.from,
      notify_to: settings.smtp.notify_to,
      timeout_ms: settings.smtp.timeout_ms,
      password_input: "",
    },
    rateLimits: settings.rate_limits,
    turnstile: {
      enabled: settings.turnstile.enabled,
      site_key: settings.turnstile.site_key,
      hostnames_input: settings.turnstile.hostnames.join(", "),
      timeout_ms: settings.turnstile.timeout_ms,
      secret_input: "",
    },
    spam: settings.spam,
    aiSpam: {
      enabled: settings.ai_spam.enabled,
      account_id: settings.ai_spam.account_id,
      model: settings.ai_spam.model,
      style: settings.ai_spam.style,
      timeout_ms: settings.ai_spam.timeout_ms,
      threshold: settings.ai_spam.threshold,
      daily_limit: settings.ai_spam.daily_limit,
      max_chars: settings.ai_spam.max_chars,
      token_input: "",
    },
  };
}

const SECRET = "__clear__";

function SecretField({
  id,
  label,
  state,
  value,
  onChange,
  disabled,
}: {
  id: string;
  label: string;
  state: { configured: boolean; readable: boolean };
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  return (
    <label htmlFor={id} className="flex flex-col gap-1 text-[12px]">
      {label}
      <span className="flex items-center gap-2">
        <Input
          id={id}
          type="password"
          className="flex-1"
          disabled={disabled}
          value={value}
          placeholder={
            state.configured
              ? state.readable
                ? "已配置（加密存储）——留空保持不变"
                : "已配置但当前不可读——请重新输入"
              : "未配置"
          }
          onChange={(event) => onChange(event.target.value)}
          autoComplete="new-password"
        />
        {state.configured && (
          <Button
            type="button"
            className="px-2"
            disabled={disabled || value === SECRET}
            onClick={() => onChange(SECRET)}
          >
            清除
          </Button>
        )}
      </span>
      {value === SECRET && <span className="text-[11px] text-[#7a4b00]">保存后将清除该密钥。</span>}
    </label>
  );
}

function NumField({
  id,
  label,
  value,
  min,
  max,
  onChange,
}: {
  id: string;
  label: string;
  value: number;
  min?: number;
  max?: number;
  onChange: (value: number) => void;
}) {
  return (
    <label htmlFor={id} className="flex flex-col gap-1 text-[12px]">
      {label}
      <Input
        id={id}
        type="number"
        value={value}
        min={min}
        max={max}
        className="w-32"
        onChange={(event) => {
          const parsed = Number(event.target.value);
          if (Number.isFinite(parsed)) onChange(parsed);
        }}
      />
    </label>
  );
}

const RATE_LIMIT_FIELDS: { key: keyof RateLimitSettings; label: string }[] = [
  { key: "submit_hourly", label: "投稿 / 小时 / IP" },
  { key: "submit_daily", label: "投稿 / 天 / IP" },
  { key: "resend_hourly", label: "确认邮件 / 小时 / 邮箱" },
  { key: "resend_daily", label: "确认邮件 / 天 / 邮箱" },
  { key: "confirm_hourly", label: "确认请求 / 小时 / 邮箱" },
  { key: "confirm_daily", label: "确认请求 / 天 / 邮箱" },
  { key: "confirm_daily_global", label: "确认请求 / 天（全局）" },
  { key: "admin_mail_hourly", label: "管理员通知 / 小时" },
];

export function SettingsPanel() {
  const { logout } = useAdminSession();
  const [settings, setSettings] = useState<RuntimeSettings | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState("");
  const [conflict, setConflict] = useState(false);
  const [audit, setAudit] = useState<SettingsAuditRecord[]>([]);
  const [auditTotal, setAuditTotal] = useState(0);
  const [auditPage, setAuditPage] = useState(1);

  const reload = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [settingsResponse, auditResponse] = await Promise.all([
        getSettings(),
        listSettingsAudit({ page: 1, pageSize: 10 }),
      ]);
      setSettings(settingsResponse.data);
      setDraft(draftFrom(settingsResponse.data));
      setAudit(auditResponse.data);
      setAuditTotal(auditResponse.meta.total);
      setAuditPage(1);
      setConflict(false);
      setFieldErrors({});
    } catch (reason) {
      setError(reason instanceof FeedbackApiError ? reason.message : "无法读取设置。");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const loadAuditPage = useCallback(async (page: number) => {
    try {
      const response = await listSettingsAudit({ page, pageSize: 10 });
      setAudit(response.data);
      setAuditTotal(response.meta.total);
      setAuditPage(page);
    } catch {
      // 审计翻页失败不阻塞主界面
    }
  }, []);

  async function save() {
    if (!settings || !draft || saving) return;
    setSaving(true);
    setError("");
    setNotice("");
    setConflict(false);
    setFieldErrors({});
    try {
      const response = await updateSettings({
        base_revision: settings.revision,
        subscriptions_enabled: draft.subscriptions,
        smtp: {
          enabled: draft.smtp.enabled,
          host: draft.smtp.host.trim(),
          port: draft.smtp.port,
          tls_mode: draft.smtp.tls_mode,
          username: draft.smtp.username.trim(),
          from: draft.smtp.from.trim(),
          notify_to: draft.smtp.notify_to.trim(),
          timeout_ms: draft.smtp.timeout_ms,
          ...(draft.smtp.password_input ? { password: draft.smtp.password_input } : {}),
        },
        rate_limits: draft.rateLimits,
        turnstile: {
          enabled: draft.turnstile.enabled,
          site_key: draft.turnstile.site_key.trim(),
          timeout_ms: draft.turnstile.timeout_ms,
          hostnames: draft.turnstile.hostnames_input
            .split(",")
            .map((host) => host.trim().toLowerCase())
            .filter(Boolean),
          ...(draft.turnstile.secret_input ? { secret_key: draft.turnstile.secret_input } : {}),
        },
        spam: draft.spam,
        ai_spam: {
          enabled: draft.aiSpam.enabled,
          account_id: draft.aiSpam.account_id.trim(),
          model: draft.aiSpam.model.trim(),
          style: draft.aiSpam.style,
          timeout_ms: draft.aiSpam.timeout_ms,
          threshold: draft.aiSpam.threshold,
          daily_limit: draft.aiSpam.daily_limit,
          max_chars: draft.aiSpam.max_chars,
          ...(draft.aiSpam.token_input ? { api_token: draft.aiSpam.token_input } : {}),
        },
      });
      setSettings(response.data);
      setDraft(draftFrom(response.data));
      setNotice("已保存并立即生效，无需重启。");
      await loadAuditPage(1);
    } catch (reason) {
      if (reason instanceof FeedbackApiError) {
        if (reason.code === "SETTINGS_REVISION_CONFLICT") {
          setConflict(true);
          setError("设置已被其他会话修改。请点击「重新加载」后再修改，避免覆盖对方的变更。");
        } else if (reason.code === "SETTINGS_SECRET_UNWRITABLE") {
          setError("服务器未配置 BIODAYS_SETTINGS_ENC_KEY，无法保存密钥；其余设置可正常保存。");
        } else {
          setError(reason.message);
        }
        if (Object.keys(reason.fields).length > 0) setFieldErrors(reason.fields);
      } else {
        setError("保存失败，请稍后重试。");
      }
    } finally {
      setSaving(false);
    }
  }

  if (loading && !settings) {
    return (
      <p role="status" className="text-[12px]">
        正在读取设置…
      </p>
    );
  }
  if (!settings || !draft) {
    return (
      <div className="flex flex-col gap-2">
        {error && (
          <p role="alert" className="border border-[#a00000] bg-[#ffe8e8] px-3 py-2 text-[12px]">
            {error}
          </p>
        )}
        <Button type="button" className="w-32" onClick={() => void reload()}>
          重新加载
        </Button>
      </div>
    );
  }

  const smtpEnabled = draft.smtp.enabled;
  const fieldError = (name: string) =>
    fieldErrors[name] ? (
      <span role="alert" className="text-[11px] text-[#7a0000]">
        {fieldErrors[name]}
      </span>
    ) : null;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <Button as={Link} href="/admin/mail" className="px-3">
          ← 邮件队列
        </Button>
        <Button type="button" className="px-3" onClick={() => void reload()}>
          重新加载
        </Button>
        <Button type="button" className="px-3" onClick={() => void save()} disabled={saving}>
          {saving ? "保存中…" : "保存全部修改"}
        </Button>
        <Button type="button" className="px-3" onClick={() => void logout()}>
          退出
        </Button>
        <span className="text-[11px] text-[#555]">
          revision {settings.revision}
          {settings.updated_at.startsWith("0001")
            ? ""
            : ` · ${formatDateTime(settings.updated_at)} 由 ${settings.updated_by || "—"} 更新`}
        </span>
      </div>

      {!settings.encryption_key_configured && (
        <Frame
          boxShadow="$in"
          padding="$2"
          className="border-2 border-[#7a4b00] text-[12px] text-[#7a4b00]"
        >
          服务器未设置 BIODAYS_SETTINGS_ENC_KEY（32 字节 base64，openssl rand -base64
          32）。密钥字段无法保存； 添加该环境变量并重启一次后即可在后台管理全部密钥。
        </Frame>
      )}
      {conflict && (
        <Frame
          boxShadow="$in"
          padding="$2"
          className="border-2 border-[#a00000] text-[12px] text-[#7a0000]"
        >
          {error}
        </Frame>
      )}
      {!conflict && error && (
        <p role="alert" className="border border-[#a00000] bg-[#ffe8e8] px-3 py-2 text-[12px]">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="border border-[#2d6a2d] bg-[#e6ffe6] px-3 py-2 text-[12px]">
          {notice}
        </p>
      )}

      {/* ── 邮件 SMTP ── */}
      <Frame boxShadow="$out" padding="$3" className="text-[12px]">
        <h2 className="mb-2 font-bold">邮件（SMTP）</h2>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <Checkbox
              checked={draft.smtp.enabled}
              onChange={(event) =>
                setDraft({ ...draft, smtp: { ...draft.smtp, enabled: event.target.checked } })
              }
              label="启用 SMTP（关闭时反馈仍可提交，邮件任务保留在队列）"
            />
          </div>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <label htmlFor="smtp-host" className="flex flex-col gap-1">
              服务器地址
              <Input
                id="smtp-host"
                value={draft.smtp.host}
                disabled={!smtpEnabled}
                onChange={(event) =>
                  setDraft({ ...draft, smtp: { ...draft.smtp, host: event.target.value } })
                }
              />
              {fieldError("smtp.host")}
            </label>
            <NumField
              id="smtp-port"
              label="端口"
              value={draft.smtp.port}
              min={1}
              max={65535}
              onChange={(port) => setDraft({ ...draft, smtp: { ...draft.smtp, port } })}
            />
            <label htmlFor="smtp-tls" className="flex flex-col gap-1">
              TLS 模式（587=starttls，465=implicit_tls）
              <Dropdown
                id="smtp-tls"
                disabled={!smtpEnabled}
                options={["starttls", "implicit_tls"]}
                value={draft.smtp.tls_mode}
                onChange={(event) =>
                  setDraft({
                    ...draft,
                    smtp: {
                      ...draft.smtp,
                      tls_mode: event.target.value as "starttls" | "implicit_tls",
                    },
                  })
                }
              />
              {fieldError("smtp.tls_mode")}
            </label>
            <label htmlFor="smtp-username" className="flex flex-col gap-1">
              用户名
              <Input
                id="smtp-username"
                value={draft.smtp.username}
                disabled={!smtpEnabled}
                autoComplete="off"
                onChange={(event) =>
                  setDraft({ ...draft, smtp: { ...draft.smtp, username: event.target.value } })
                }
              />
            </label>
            <SecretField
              id="smtp-password"
              label="密码 / 授权码"
              state={{ configured: settings.smtp.password.configured, readable: true }}
              value={draft.smtp.password_input}
              disabled={!smtpEnabled}
              onChange={(value) =>
                setDraft({ ...draft, smtp: { ...draft.smtp, password_input: value } })
              }
            />
            {fieldError("smtp.password")}
            <label htmlFor="smtp-from" className="flex flex-col gap-1">
              发件地址
              <Input
                id="smtp-from"
                type="email"
                value={draft.smtp.from}
                disabled={!smtpEnabled}
                onChange={(event) =>
                  setDraft({ ...draft, smtp: { ...draft.smtp, from: event.target.value } })
                }
              />
              {fieldError("smtp.from")}
            </label>
            <label htmlFor="smtp-notify" className="flex flex-col gap-1">
              管理员通知收件地址
              <Input
                id="smtp-notify"
                type="email"
                value={draft.smtp.notify_to}
                disabled={!smtpEnabled}
                onChange={(event) =>
                  setDraft({ ...draft, smtp: { ...draft.smtp, notify_to: event.target.value } })
                }
              />
              {fieldError("smtp.notify_to")}
            </label>
            <NumField
              id="smtp-timeout"
              label="超时（毫秒，1000–60000）"
              value={draft.smtp.timeout_ms}
              min={1000}
              max={60000}
              onChange={(timeout) =>
                setDraft({ ...draft, smtp: { ...draft.smtp, timeout_ms: timeout } })
              }
            />
          </div>
          <p className="text-[11px] text-[#555]">
            修改立即生效。保存后可到「邮件队列」页发送测试邮件验证连通性。
          </p>
        </div>
      </Frame>

      {/* ── 频控限额 ── */}
      <Frame boxShadow="$out" padding="$3" className="text-[12px]">
        <h2 className="mb-2 font-bold">频控限额</h2>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
          {RATE_LIMIT_FIELDS.map((item) => (
            <div key={item.key} className="flex flex-col gap-1">
              <NumField
                id={`limit-${item.key}`}
                label={item.label}
                value={draft.rateLimits[item.key]}
                min={1}
                max={10000}
                onChange={(value) =>
                  setDraft({ ...draft, rateLimits: { ...draft.rateLimits, [item.key]: value } })
                }
              />
              {fieldError(`rate_limits.${item.key}`)}
            </div>
          ))}
        </div>
        <p className="mt-2 text-[11px] text-[#555]">
          保存后新请求立即按新限额执行；已消耗的窗口不会重置。
        </p>
      </Frame>

      {/* ── Turnstile ── */}
      <Frame boxShadow="$out" padding="$3" className="text-[12px]">
        <h2 className="mb-2 font-bold">Turnstile 人机验证</h2>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <Checkbox
              checked={draft.turnstile.enabled}
              onChange={(event) =>
                setDraft({
                  ...draft,
                  turnstile: { ...draft.turnstile, enabled: event.target.checked },
                })
              }
              label="启用 Turnstile（官方不支持中国大陆；大陆入口请保持关闭）"
            />
          </div>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <label htmlFor="turnstile-site" className="flex flex-col gap-1">
              Site Key（公开）
              <Input
                id="turnstile-site"
                value={draft.turnstile.site_key}
                disabled={!draft.turnstile.enabled}
                onChange={(event) =>
                  setDraft({
                    ...draft,
                    turnstile: { ...draft.turnstile, site_key: event.target.value },
                  })
                }
              />
              {fieldError("turnstile.site_key")}
            </label>
            <SecretField
              id="turnstile-secret"
              label="Secret Key"
              state={{ configured: settings.turnstile.secret_key.configured, readable: true }}
              value={draft.turnstile.secret_input}
              disabled={!draft.turnstile.enabled}
              onChange={(value) =>
                setDraft({ ...draft, turnstile: { ...draft.turnstile, secret_input: value } })
              }
            />
            {fieldError("turnstile.secret_key")}
            <label htmlFor="turnstile-hosts" className="flex flex-col gap-1">
              Hostnames（逗号分隔，留空用站点域名）
              <Input
                id="turnstile-hosts"
                value={draft.turnstile.hostnames_input}
                disabled={!draft.turnstile.enabled}
                onChange={(event) =>
                  setDraft({
                    ...draft,
                    turnstile: { ...draft.turnstile, hostnames_input: event.target.value },
                  })
                }
              />
              {fieldError("turnstile.hostnames")}
            </label>
            <NumField
              id="turnstile-timeout"
              label="验证超时（毫秒，1000–10000）"
              value={draft.turnstile.timeout_ms}
              min={1000}
              max={10000}
              onChange={(timeout) =>
                setDraft({ ...draft, turnstile: { ...draft.turnstile, timeout_ms: timeout } })
              }
            />
          </div>
        </div>
      </Frame>

      {/* ── 垃圾评分 ── */}
      <Frame boxShadow="$out" padding="$3" className="text-[12px]">
        <h2 className="mb-2 font-bold">垃圾评分</h2>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <Checkbox
              checked={draft.spam.enabled}
              onChange={(event) =>
                setDraft({ ...draft, spam: { ...draft.spam, enabled: event.target.checked } })
              }
              label="启用规则评分与自动隔离"
            />
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="flex flex-col gap-1">
              <NumField
                id="spam-review"
                label="待复核阈值（1–99）"
                value={draft.spam.review_score}
                min={1}
                max={99}
                onChange={(value) =>
                  setDraft({ ...draft, spam: { ...draft.spam, review_score: value } })
                }
              />
              {fieldError("spam.review_score")}
            </div>
            <div className="flex flex-col gap-1">
              <NumField
                id="spam-block"
                label="自动隔离阈值（50–100）"
                value={draft.spam.block_score}
                min={50}
                max={100}
                onChange={(value) =>
                  setDraft({ ...draft, spam: { ...draft.spam, block_score: value } })
                }
              />
            </div>
          </div>
          <p className="text-[11px] text-[#555]">
            0–{draft.spam.review_score} 正常，{draft.spam.review_score}–{draft.spam.block_score}{" "}
            待复核，≥{draft.spam.block_score} 自动隔离。
          </p>
        </div>
      </Frame>

      {/* ── Workers AI 复核 ── */}
      <Frame boxShadow="$out" padding="$3" className="text-[12px]">
        <h2 className="mb-2 font-bold">Workers AI 复核（可选）</h2>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <Checkbox
              checked={draft.aiSpam.enabled}
              disabled={!draft.spam.enabled}
              onChange={(event) =>
                setDraft({ ...draft, aiSpam: { ...draft.aiSpam, enabled: event.target.checked } })
              }
              label="把「待复核」投稿的去标识文本交给 Cloudflare Workers AI 判断"
            />
          </div>
          {!draft.spam.enabled && (
            <p className="text-[11px] text-[#7a4b00]">需要先启用规则评分。</p>
          )}
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <label htmlFor="ai-account" className="flex flex-col gap-1">
              Cloudflare Account ID
              <Input
                id="ai-account"
                value={draft.aiSpam.account_id}
                disabled={!draft.aiSpam.enabled}
                onChange={(event) =>
                  setDraft({
                    ...draft,
                    aiSpam: { ...draft.aiSpam, account_id: event.target.value },
                  })
                }
              />
              {fieldError("ai_spam.account_id")}
            </label>
            <SecretField
              id="ai-token"
              label="API Token"
              state={{ configured: settings.ai_spam.api_token.configured, readable: true }}
              value={draft.aiSpam.token_input}
              disabled={!draft.aiSpam.enabled}
              onChange={(value) =>
                setDraft({ ...draft, aiSpam: { ...draft.aiSpam, token_input: value } })
              }
            />
            {fieldError("ai_spam.api_token")}
            <label htmlFor="ai-model" className="flex flex-col gap-1">
              模型
              <Input
                id="ai-model"
                value={draft.aiSpam.model}
                disabled={!draft.aiSpam.enabled}
                onChange={(event) =>
                  setDraft({ ...draft, aiSpam: { ...draft.aiSpam, model: event.target.value } })
                }
              />
            </label>
            <label htmlFor="ai-style" className="flex flex-col gap-1">
              请求形态
              <Dropdown
                id="ai-style"
                disabled={!draft.aiSpam.enabled}
                options={["decision", "chat"]}
                value={draft.aiSpam.style}
                onChange={(event) =>
                  setDraft({
                    ...draft,
                    aiSpam: { ...draft.aiSpam, style: event.target.value as "decision" | "chat" },
                  })
                }
              />
            </label>
            <NumField
              id="ai-timeout"
              label="超时（毫秒，1000–10000）"
              value={draft.aiSpam.timeout_ms}
              min={1000}
              max={10000}
              onChange={(timeout) =>
                setDraft({ ...draft, aiSpam: { ...draft.aiSpam, timeout_ms: timeout } })
              }
            />
            <NumField
              id="ai-threshold"
              label="判定阈值（0.5–1，按 ×1000 输入）"
              value={Math.round(draft.aiSpam.threshold * 1000)}
              min={500}
              max={1000}
              onChange={(value) =>
                setDraft({ ...draft, aiSpam: { ...draft.aiSpam, threshold: value / 1000 } })
              }
            />
            <NumField
              id="ai-limit"
              label="每日额度（1–10000）"
              value={draft.aiSpam.daily_limit}
              min={1}
              max={10000}
              onChange={(value) =>
                setDraft({ ...draft, aiSpam: { ...draft.aiSpam, daily_limit: value } })
              }
            />
            <NumField
              id="ai-chars"
              label="送审字符上限（500–8000）"
              value={draft.aiSpam.max_chars}
              min={500}
              max={8000}
              onChange={(value) =>
                setDraft({ ...draft, aiSpam: { ...draft.aiSpam, max_chars: value } })
              }
            />
          </div>
        </div>
      </Frame>

      {/* ── 功能开关 ── */}
      <Frame boxShadow="$out" padding="$3" className="text-[12px]">
        <h2 className="mb-2 font-bold">功能开关</h2>
        <div className="flex items-center gap-2">
          <Checkbox
            checked={draft.subscriptions}
            onChange={(event) => setDraft({ ...draft, subscriptions: event.target.checked })}
            label="允许访客订阅处理结果邮件（需要 SMTP 已启用并配置完整）"
          />
        </div>
      </Frame>

      {/* ── 审计 ── */}
      <Frame boxShadow="$out" padding="$3" className="text-[12px]">
        <h2 className="mb-2 font-bold">修改记录</h2>
        <Frame boxShadow="$in" className="overflow-x-auto bg-white">
          <table className="w-full min-w-[560px] border-collapse text-left text-[12px]">
            <thead>
              <tr className="bg-[#c0c0c0]">
                <th className="px-2 py-1">时间</th>
                <th className="px-2 py-1">操作者</th>
                <th className="px-2 py-1">Revision</th>
                <th className="px-2 py-1">变更</th>
              </tr>
            </thead>
            <tbody>
              {audit.map((record) => (
                <tr key={record.id} className="align-top odd:bg-[#f7f7f7]">
                  <td className="px-2 py-1 whitespace-nowrap">
                    {formatDateTime(record.created_at)}
                  </td>
                  <td className="px-2 py-1 whitespace-nowrap">{record.actor}</td>
                  <td className="px-2 py-1">{record.revision}</td>
                  <td className="max-w-[26rem] px-2 py-1 font-mono text-[10px] break-all">
                    {summarizeChanges(record.changes)}
                  </td>
                </tr>
              ))}
              {audit.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-2 py-4 text-center">
                    暂无修改记录。
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </Frame>
        <div className="mt-2 flex items-center gap-2">
          <Button
            type="button"
            className="px-2"
            disabled={auditPage <= 1}
            onClick={() => void loadAuditPage(auditPage - 1)}
          >
            上一页
          </Button>
          <span className="text-[11px]">
            第 {auditPage} / {Math.max(1, Math.ceil(auditTotal / 10))} 页
          </span>
          <Button
            type="button"
            className="px-2"
            disabled={auditPage >= Math.ceil(auditTotal / 10)}
            onClick={() => void loadAuditPage(auditPage + 1)}
          >
            下一页
          </Button>
        </div>
      </Frame>
    </div>
  );
}

function summarizeChanges(raw: string): string {
  try {
    const parsed: Record<string, { old: unknown; new: unknown }> = JSON.parse(raw);
    return Object.entries(parsed)
      .map(([field, change]) => `${field}: ${String(change.old)} → ${String(change.new)}`)
      .join("; ");
  } catch {
    return raw;
  }
}
