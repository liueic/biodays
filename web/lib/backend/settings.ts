import { check, db, HttpError, rpc } from "./db";
import {
  deploymentStatus,
  gatewayKey,
  resendEnvironment,
  turnstileEnvironment,
} from "./deployment";

export const JEV_MODEL = "typesafe-ai/jev-1.14.0";
export const isJevModel = (value: unknown): value is string =>
  typeof value === "string" && /^typesafe-ai\/jev(?:-\d+\.\d+\.\d+)?$/.test(value);

export const defaults = () => ({
  subscriptions_enabled: true,
  resend: {
    enabled: false,
  },
  rate_limits: {
    submit_hourly: 5,
    submit_daily: 20,
    resend_hourly: 3,
    resend_daily: 10,
    confirm_hourly: 1,
    confirm_daily: 3,
    confirm_daily_global: 100,
    admin_mail_hourly: 10,
  },
  turnstile: {
    enabled: false,
    timeout_ms: 5000,
  },
  spam: { enabled: true, review_score: 30, block_score: 70 },
  ai_spam: {
    enabled: false,
    model: JEV_MODEL,
    timeout_ms: 5000,
    threshold: 0.8,
    daily_limit: 500,
    max_chars: 4000,
  },
});
export type Settings = ReturnType<typeof defaults>;
export type SettingsRow = {
  revision: number;
  document: Settings;
  updated_at: string;
  updated_by: string;
};
const deploymentPaths = new Set([
  "resend.api_key",
  "resend.from",
  "resend.notify_to",
  "turnstile.site_key",
  "turnstile.secret_key",
  "turnstile.hostnames",
  "ai_spam.api_key",
]);

// Only business settings belong in the database. Historic deployment values and
// encrypted keys are ignored; reads never mutate the stored revision.
export function normalizeSettings(document: Record<string, unknown>): Settings {
  const normalized = defaults();
  for (const group of Object.keys(normalized) as (keyof Settings)[]) {
    if (group === "subscriptions_enabled") {
      if (typeof document[group] === "boolean") normalized[group] = document[group];
      continue;
    }
    const raw = document[group];
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) continue;
    const source = raw as Record<string, unknown>;
    const target = normalized[group] as Record<string, unknown>;
    const legacy =
      group === "ai_spam" &&
      (!isJevModel(source.model) ||
        Object.hasOwn(source, "account_id") ||
        Object.hasOwn(source, "api_token") ||
        Object.hasOwn(source, "style"));
    for (const key of Object.keys(target)) {
      if (legacy && !["timeout_ms", "threshold", "daily_limit", "max_chars"].includes(key))
        continue;
      if (typeof source[key] === typeof target[key]) target[key] = source[key];
    }
  }
  // Old confidence thresholds could include values that overlap Boolean bands.
  if (normalized.ai_spam.threshold <= 0.5 || normalized.ai_spam.threshold > 1)
    normalized.ai_spam.threshold = 0.8;
  return normalized;
}
export async function settings(): Promise<SettingsRow> {
  const client = db();
  let result = await client.from("biodays_settings").select("*").eq("id", true).maybeSingle();
  check(result.error);
  if (!result.data) {
    const seeded = await client
      .from("biodays_settings")
      .upsert({ id: true, document: defaults() }, { onConflict: "id", ignoreDuplicates: true });
    check(seeded.error);
    result = await client.from("biodays_settings").select("*").eq("id", true).single();
    check(result.error);
  }
  const row = result.data as SettingsRow;
  return { ...row, document: normalizeSettings(row.document) };
}
export function publicSettings(row: SettingsRow) {
  return {
    ...normalizeSettings(row.document),
    revision: row.revision,
    updated_at: row.updated_at,
    updated_by: row.updated_by,
    deployment: deploymentStatus(),
  };
}
export function jevConfiguration(config: Settings["ai_spam"]) {
  const apiKey = gatewayKey();
  return { apiKey, configured: !!apiKey && isJevModel(config.model) };
}
export function mailConfiguration(s: Settings) {
  const config = resendEnvironment();
  return {
    ...config,
    enabled: s.resend.enabled,
    error: !s.resend.enabled
      ? "邮件发送已关闭。"
      : config.invalid.length
        ? `邮件环境变量格式无效：${config.invalid.join("、")}。`
        : config.missing.length
          ? "邮件环境变量配置不完整。"
          : "",
  };
}
export async function saveSettings(input: Record<string, unknown>, actor: string) {
  const row = await settings(),
    next = structuredClone(row.document);
  if (!Number.isInteger(input.base_revision))
    throw new HttpError(400, "INVALID_PARAM", "缺少配置版本。");
  for (const [group, value] of Object.entries(input)) {
    if (group === "base_revision") continue;
    if (group === "deployment")
      throw new HttpError(422, "READ_ONLY_SETTING", "部署配置由环境变量管理，后台只读。");
    if (group === "subscriptions_enabled") {
      if (typeof value !== "boolean")
        throw new HttpError(422, "VALIDATION_FAILED", "订阅开关必须为布尔值。");
      next[group] = value;
      continue;
    }
    if (!Object.hasOwn(next, group) || !value || typeof value !== "object" || Array.isArray(value))
      throw new HttpError(422, "VALIDATION_FAILED", `未知配置：${group}`);
    const section = next[group as keyof Settings] as Record<string, unknown>;
    for (const [key, v] of Object.entries(value)) {
      if (deploymentPaths.has(`${group}.${key}`))
        throw new HttpError(422, "READ_ONLY_SETTING", `${group}.${key} 由环境变量管理，后台只读。`);
      if (!Object.hasOwn(section, key))
        throw new HttpError(422, "VALIDATION_FAILED", `未知配置：${group}.${key}`);
      if (typeof section[key] !== typeof v)
        throw new HttpError(422, "VALIDATION_FAILED", `配置类型无效：${group}.${key}`);
      section[key] = typeof v === "string" ? v.trim() : v;
    }
  }
  const invalid = (message: string): never => {
    throw new HttpError(422, "VALIDATION_FAILED", message);
  };
  for (const v of Object.values(next.rate_limits))
    if (!Number.isInteger(v) || v < 1 || v > 100000) invalid("频控必须为 1–100000 的整数。");
  if (
    ![next.spam.review_score, next.spam.block_score].every(
      (n) => Number.isInteger(n) && n >= 0 && n <= 100,
    ) ||
    next.spam.review_score >= next.spam.block_score
  )
    invalid("垃圾信息阈值无效。");
  for (const n of [next.turnstile.timeout_ms, next.ai_spam.timeout_ms])
    if (!Number.isInteger(n) || n < 1000 || n > 15000) invalid("验证超时必须为 1000–15000 毫秒。");
  if (next.turnstile.enabled && !turnstileEnvironment().configured)
    invalid("请先在环境变量中完整配置 Turnstile 密钥与允许的域名。");
  if (
    !Number.isFinite(next.ai_spam.threshold) ||
    next.ai_spam.threshold <= 0.5 ||
    next.ai_spam.threshold > 1 ||
    !Number.isInteger(next.ai_spam.daily_limit) ||
    next.ai_spam.daily_limit < 1 ||
    next.ai_spam.daily_limit > 100000 ||
    !Number.isInteger(next.ai_spam.max_chars) ||
    next.ai_spam.max_chars < 100 ||
    next.ai_spam.max_chars > 5000
  )
    invalid("AI 参数无效。");
  if (!isJevModel(next.ai_spam.model)) invalid("请选择 typesafe-ai/jev 或固定版本的 Jev 模型。");
  if (next.ai_spam.enabled && (!next.spam.enabled || !jevConfiguration(next.ai_spam).configured))
    invalid("请启用垃圾信息规则，并在环境变量中配置 AI_GATEWAY_API_KEY。");
  if (next.resend.enabled && !mailConfiguration(next).configured)
    invalid("请先在环境变量中完整配置 Resend 密钥、发件地址与通知邮箱。");
  const changes: Record<string, unknown> = {};
  for (const key of Object.keys(next))
    if (
      JSON.stringify(row.document[key as keyof Settings]) !==
      JSON.stringify(next[key as keyof Settings])
    )
      changes[key] = {
        old: row.document[key as keyof Settings],
        new: next[key as keyof Settings],
      };
  // Only business settings are persisted or included in the audit log.
  try {
    return publicSettings(
      await rpc<SettingsRow>("save_settings", {
        p_base: input.base_revision,
        p_document: next,
        p_actor: actor,
        p_changes: JSON.stringify(changes),
      }),
    );
  } catch (error) {
    if (error instanceof HttpError && error.code === "CONFLICT")
      throw new HttpError(
        409,
        "SETTINGS_REVISION_CONFLICT",
        "设置已被其他管理员修改，请刷新后重试。",
      );
    throw error;
  }
}
