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
      timeout_ms: "超时（毫秒）",
    },
  },
  spam: {
    label: "垃圾信息规则",
    fields: { enabled: "启用规则判断", review_score: "复核阈值", block_score: "垃圾阈值" },
  },
  ai_spam: {
    label: "Jev · Vercel AI Gateway（仅复核可疑内容）",
    fields: {
      enabled: "启用 AI 复核",
      model: "模型",
      timeout_ms: "超时（毫秒）",
      threshold: "垃圾概率上限（大于 0.5，最大 1）",
      daily_limit: "每日调用上限",
      max_chars: "脱敏内容最大字符数",
    },
  },
};
export function SettingsPanel() {
  const [settings, setSettings] = useState<RuntimeSettings | null>(null),
    [draft, setDraft] = useState<Draft>({}),
    [audit, setAudit] = useState<SettingsAuditRecord[]>([]),
    [notice, setNotice] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [refreshing, setRefreshing] = useState(false);
  const initialize = useCallback((s: RuntimeSettings) => {
    setSettings(s);
    const values: Draft = { subscriptions_enabled: s.subscriptions_enabled };
    for (const [group, { fields }] of Object.entries(groups))
      for (const key of Object.keys(fields)) {
        const value = (s[group as keyof RuntimeSettings] as unknown as Record<string, unknown>)[
          key
        ];
        values[`${group}.${key}`] = value as string | number | boolean;
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
  async function refreshStatus() {
    setRefreshing(true);
    setError("");
    try {
      const fresh = (await getSettings()).data;
      // Preserve the form revision and draft so refresh cannot bypass CAS checks.
      setSettings((current) => (current ? { ...current, deployment: fresh.deployment } : fresh));
    } catch (e) {
      setError(e instanceof Error ? e.message : "状态读取失败");
    } finally {
      setRefreshing(false);
    }
  }
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
        section[key] = value;
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
      <Fieldset legend="部署配置（只读）">
        <div className="flex flex-col gap-3">
          <p className="reading">
            密钥、站点地址与服务连接信息由环境变量管理。线上请在 Vercel 修改并重新部署，本地请修改
            web/.env.local 并重启服务。
          </p>
          <p className="reading">
            当前环境：{settings.deployment.environment}
            。这里检查配置是否齐全及格式是否有效，不会发送邮件或调用 AI，不能代替服务连通性测试。
          </p>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-[12px]" aria-label="环境变量配置状态">
              <thead>
                <tr>
                  <th className="p-2">配置</th>
                  <th className="p-2">状态</th>
                  <th className="p-2">当前值</th>
                </tr>
              </thead>
              <tbody>
                {settings.deployment.checks.map((check) => (
                  <tr
                    key={check.name}
                    className="border-t border-[#aaa]"
                    data-environment-variable={check.name}
                  >
                    <td className="p-2">
                      <div>{check.label}</div>
                      <code className="break-all">{check.name}</code>
                    </td>
                    <td className="p-2">
                      {
                        {
                          configured: "已配置",
                          missing: "未配置",
                          invalid: "格式无效",
                          default: "默认规则",
                        }[check.status]
                      }
                    </td>
                    <td className="p-2 break-all">
                      {check.secret
                        ? check.status === "configured"
                          ? "已配置（隐藏）"
                          : "—"
                        : check.value || "—"}
                      {check.message && <p>{check.message}</p>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="reading">
            配置完整性：Resend {settings.deployment.services.resend.configured ? "完整" : "不完整"}
            ； AI Gateway {settings.deployment.services.ai_gateway.configured ? "完整" : "不完整"}；
            Turnstile {settings.deployment.services.turnstile.configured ? "完整" : "不完整"}。
            配置齐全后可在下方启用对应功能。
          </p>
          <Button type="button" disabled={refreshing || busy} onClick={() => void refreshStatus()}>
            {refreshing ? "读取中…" : "刷新配置状态"}
          </Button>
        </div>
      </Fieldset>
      <p className="reading">
        业务配置版本 {settings.revision}。保存后下一次请求生效，无需重新部署。
      </p>
      <p className="reading">
        Jev 的垃圾概率达到上限时标记为垃圾，低于或等于 1 − 上限时判为正常，中间值保留人工审核。
      </p>
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
                value = draft[path];
              return (
                <label key={path} className="flex flex-col gap-1 text-[12px]" htmlFor={path}>
                  {label}
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
                      type={typeof value === "number" ? "number" : "text"}
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
      <Button type="button" disabled={busy || refreshing} onClick={() => void save()}>
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
