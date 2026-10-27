"use client";
import { Button } from "@react95/core/Button";
import { Fieldset } from "@react95/core/Fieldset";
import { Input } from "@react95/core/Input";
import { useCallback, useEffect, useState } from "react";
import {
  getSettings,
  listSettingsAudit,
  type RuntimeSettings,
  type SettingsAuditRecord,
  type SettingsPatch,
  updateSettings,
} from "@/lib/admin-api";

type Draft = Record<string, string | number | boolean>;
const groups: Record<string, { label: string; fields: Record<string, string> }> = {
  resend: {
    label: "Resend 邮件",
    fields: {
      enabled: "启用发送",
      from: "发件地址（已在 Resend 验证的域名）",
      notify_to: "管理员通知邮箱",
      api_key: "API Key",
    },
  },
  rate_limits: {
    label: "频率限制",
    fields: {
      submit_hourly: "提交 / 小时",
      submit_daily: "提交 / 天",
      resend_hourly: "重发确认邮件发送 / 小时",
      resend_daily: "重发确认邮件发送 / 天",
      confirm_hourly: "确认邮件发送 / 小时",
      confirm_daily: "确认邮件发送 / 天",
      confirm_daily_global: "确认邮件发送总量 / 天",
      admin_mail_hourly: "管理员邮件操作 / 小时",
    },
  },
  turnstile: {
    label: "Turnstile",
    fields: {
      enabled: "启用人机验证",
      site_key: "Site Key",
      secret_key: "Secret Key",
      hostnames: "允许的主机名（逗号分隔）",
      timeout_ms: "超时（毫秒）",
    },
  },
  spam: {
    label: "垃圾信息规则",
    fields: { enabled: "启用规则判断", review_score: "复核阈值", block_score: "垃圾阈值" },
  },
  ai_spam: {
    label: "Workers AI（仅复核可疑内容）",
    fields: {
      enabled: "启用 AI 复核",
      account_id: "Account ID",
      api_token: "API Token",
      model: "模型",
      style: "模式：chat 或 decision",
      timeout_ms: "超时（毫秒）",
      threshold: "置信度阈值",
      daily_limit: "每日调用上限",
      max_chars: "脱敏内容最大字符数",
    },
  },
};
const secrets = new Set(["resend.api_key", "turnstile.secret_key", "ai_spam.api_token"]);
export function SettingsPanel() {
  const [settings, setSettings] = useState<RuntimeSettings | null>(null),
    [draft, setDraft] = useState<Draft>({}),
    [audit, setAudit] = useState<SettingsAuditRecord[]>([]),
    [notice, setNotice] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const initialize = useCallback((s: RuntimeSettings) => {
    setSettings(s);
    const values: Draft = { subscriptions_enabled: s.subscriptions_enabled };
    for (const [group, { fields }] of Object.entries(groups))
      for (const key of Object.keys(fields)) {
        const value = (s[group as keyof RuntimeSettings] as unknown as Record<string, unknown>)[
          key
        ];
        values[`${group}.${key}`] = secrets.has(`${group}.${key}`)
          ? ""
          : Array.isArray(value)
            ? value.join(", ")
            : (value as string | number | boolean);
      }
    setDraft(values);
  }, []);
  useEffect(() => {
    getSettings()
      .then((r) => initialize(r.data))
      .catch((e) => setError(e.message));
    listSettingsAudit({ pageSize: 20 })
      .then((r) => setAudit(r.data))
      .catch((e) => setError(e.message));
  }, [initialize]);
  async function save() {
    if (!settings) return;
    setBusy(true);
    setNotice("");
    setError("");
    const patch: Record<string, unknown> = {
      base_revision: settings.revision,
      subscriptions_enabled: draft.subscriptions_enabled,
    };
    for (const [group, { fields }] of Object.entries(groups)) {
      const section: Record<string, unknown> = {};
      for (const key of Object.keys(fields)) {
        const value = draft[`${group}.${key}`];
        if (secrets.has(`${group}.${key}`) && !value) continue;
        section[key] =
          key === "hostnames"
            ? String(value)
                .split(",")
                .map((s) => s.trim())
                .filter(Boolean)
            : value;
      }
      patch[group] = section;
    }
    try {
      initialize((await updateSettings(patch as SettingsPatch)).data);
      setNotice("设置已保存，下一次请求生效。");
      setAudit((await listSettingsAudit({ pageSize: 20 })).data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }
  if (!settings) return <p role="status">{error || "正在加载设置…"}</p>;
  return (
    <div className="flex flex-col gap-4">
      <p className="reading">
        版本 {settings.revision}。密钥留空表示保留，填写 __clear__ 表示清除。密钥不会回显。
      </p>
      {!settings.encryption_key_configured && (
        <p role="status">
          保存新密钥前需配置 BIODAYS_SETTINGS_ENC_KEY。环境变量中的 Resend 密钥仍可使用。
        </p>
      )}
      <label>
        <input
          type="checkbox"
          checked={!!draft.subscriptions_enabled}
          onChange={(e) => setDraft({ ...draft, subscriptions_enabled: e.target.checked })}
        />{" "}
        允许订阅处理通知
      </label>
      {Object.entries(groups).map(([group, { label, fields }]) => (
        <Fieldset key={group} legend={label}>
          <div className="grid gap-3 md:grid-cols-2">
            {Object.entries(fields).map(([key, label]) => {
              const path = `${group}.${key}`,
                value = draft[path],
                secret = secrets.has(path),
                state = secret
                  ? (
                      settings[group as keyof RuntimeSettings] as unknown as Record<
                        string,
                        { configured: boolean; readable: boolean }
                      >
                    )[key]
                  : null;
              return (
                <label key={path} className="flex flex-col gap-1 text-[12px]" htmlFor={path}>
                  {label}
                  {state
                    ? `（${state.configured ? (state.readable ? "已配置" : "不可读取") : "未配置"}）`
                    : ""}
                  {typeof value === "boolean" ? (
                    <input
                      id={path}
                      type="checkbox"
                      checked={value}
                      onChange={(e) => setDraft({ ...draft, [path]: e.target.checked })}
                    />
                  ) : (
                    <Input
                      id={path}
                      type={secret ? "password" : typeof value === "number" ? "number" : "text"}
                      autoComplete="off"
                      value={String(value ?? "")}
                      onChange={(e) =>
                        setDraft({
                          ...draft,
                          [path]:
                            typeof value === "number" ? Number(e.target.value) : e.target.value,
                        })
                      }
                    />
                  )}
                </label>
              );
            })}
          </div>
        </Fieldset>
      ))}
      {error && (
        <p role="alert" className="text-[#a00000]">
          {error}
        </p>
      )}
      {notice && <p role="status">{notice}</p>}
      <Button type="button" disabled={busy} onClick={() => void save()}>
        {busy ? "保存中…" : "保存设置"}
      </Button>
      <Fieldset legend="最近 20 次设置变更">
        <ul>
          {audit.map((r) => (
            <li key={r.id}>
              版本 {r.revision} · {r.actor} · {new Date(r.created_at).toLocaleString("zh-CN")}
            </li>
          ))}
        </ul>
      </Fieldset>
    </div>
  );
}
