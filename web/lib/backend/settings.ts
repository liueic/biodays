import { createCipheriv, createDecipheriv, randomBytes } from "node:crypto";
import { check, db, HttpError, rpc } from "./db";

export const defaults = () => ({
  subscriptions_enabled: true,
  resend: {
    enabled: process.env.RESEND_ENABLED === "true",
    api_key: process.env.RESEND_API_KEY ? "env:RESEND_API_KEY" : "",
    from: process.env.RESEND_FROM ?? "",
    notify_to: process.env.BIODAYS_NOTIFY_TO ?? "",
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
    site_key: "",
    secret_key: "",
    hostnames: [] as string[],
    timeout_ms: 5000,
  },
  spam: { enabled: true, review_score: 30, block_score: 70 },
  ai_spam: {
    enabled: false,
    account_id: "",
    model: "@cf/cloudflare/clef-flash",
    style: "decision",
    timeout_ms: 5000,
    threshold: 0.8,
    daily_limit: 500,
    max_chars: 4000,
    api_token: "",
  },
});
export type Settings = ReturnType<typeof defaults>;
export type SettingsRow = {
  revision: number;
  document: Settings;
  updated_at: string;
  updated_by: string;
};
const secretPaths = ["resend.api_key", "turnstile.secret_key", "ai_spam.api_token"] as const;
function encryptionKey() {
  const value = process.env.BIODAYS_SETTINGS_ENC_KEY ?? "";
  const key = /^[a-f\d]{64}$/i.test(value)
    ? Buffer.from(value, "hex")
    : Buffer.from(value, "base64");
  if (key.length !== 32)
    throw new HttpError(
      422,
      "ENCRYPTION_KEY_REQUIRED",
      "请先配置 32 字节 BIODAYS_SETTINGS_ENC_KEY。",
    );
  return key;
}
export function encryptSecret(value: string) {
  if (!value) return "";
  const iv = randomBytes(12),
    cipher = createCipheriv("aes-256-gcm", encryptionKey(), iv);
  const encrypted = Buffer.concat([cipher.update(value, "utf8"), cipher.final()]);
  return `v1:${Buffer.concat([iv, cipher.getAuthTag(), encrypted]).toString("base64")}`;
}
export function decryptSecret(value: string) {
  if (!value) return "";
  if (value === "env:RESEND_API_KEY") return process.env.RESEND_API_KEY ?? "";
  if (!value.startsWith("v1:")) throw new HttpError(503, "SECRET_UNREADABLE", "密钥不可读取。");
  try {
    const raw = Buffer.from(value.slice(3), "base64");
    const decipher = createDecipheriv("aes-256-gcm", encryptionKey(), raw.subarray(0, 12));
    decipher.setAuthTag(raw.subarray(12, 28));
    return Buffer.concat([decipher.update(raw.subarray(28)), decipher.final()]).toString("utf8");
  } catch {
    throw new HttpError(503, "SECRET_UNREADABLE", "密钥不可读取，请检查加密主密钥。");
  }
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
  return result.data as SettingsRow;
}
function secretState(value: string) {
  let readable = false;
  try {
    readable = !!decryptSecret(value);
  } catch {
    /* Report state without leaking credentials. */
  }
  return { configured: !!value, encrypted: value.startsWith("v1:"), readable };
}
export function publicSettings(row: SettingsRow) {
  const document = structuredClone(row.document);
  const result: Record<string, unknown> = { ...document };
  for (const path of secretPaths) {
    const [group, key] = path.split(".");
    const section = { ...(result[group] as Record<string, unknown>) };
    section[key] = secretState(section[key] as string);
    result[group] = section;
  }
  let configured = false;
  try {
    encryptionKey();
    configured = true;
  } catch {}
  return {
    ...result,
    revision: row.revision,
    updated_at: row.updated_at,
    updated_by: row.updated_by,
    encryption_key_configured: configured,
    clear_sentinel: "__clear__",
  };
}
export function mailConfiguration(s: Settings) {
  let apiKey = "",
    error = "";
  try {
    apiKey = decryptSecret(s.resend.api_key);
  } catch {
    error = "Resend 密钥不可读取。";
  }
  const missing = [
    !apiKey && "RESEND_API_KEY",
    !s.resend.from && "RESEND_FROM",
    !s.resend.notify_to && "BIODAYS_NOTIFY_TO",
  ].filter(Boolean) as string[];
  return {
    apiKey,
    enabled: s.resend.enabled,
    configured: !missing.length && !error,
    missing,
    error:
      error || (!s.resend.enabled ? "邮件发送已关闭。" : missing.length ? "邮件配置不完整。" : ""),
  };
}
export async function saveSettings(input: Record<string, unknown>, actor: string) {
  const row = await settings(),
    next = structuredClone(row.document);
  if (!Number.isInteger(input.base_revision))
    throw new HttpError(400, "INVALID_PARAM", "缺少配置版本。");
  for (const [group, value] of Object.entries(input)) {
    if (group === "base_revision") continue;
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
      if (!Object.hasOwn(section, key))
        throw new HttpError(422, "VALIDATION_FAILED", `未知配置：${group}.${key}`);
      if ((secretPaths as readonly string[]).includes(`${group}.${key}`)) {
        if (typeof v !== "string" || v.length > 4096)
          throw new HttpError(422, "VALIDATION_FAILED", "密钥格式无效。");
        if (v) section[key] = v === "__clear__" ? "" : encryptSecret(v);
      } else if (
        typeof section[key] !== typeof v ||
        Array.isArray(section[key]) !== Array.isArray(v)
      )
        throw new HttpError(422, "VALIDATION_FAILED", `配置类型无效：${group}.${key}`);
      else section[key] = typeof v === "string" ? v.trim() : v;
    }
  }
  const invalid = (message: string): never => {
    throw new HttpError(422, "VALIDATION_FAILED", message);
  };
  const email = /^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/;
  if (
    next.resend.from &&
    (!/^[^\r\n]+$/.test(next.resend.from) ||
      !email.test(next.resend.from.match(/<([^>]+)>$/)?.[1] ?? next.resend.from))
  )
    invalid("发件地址无效。");
  if (next.resend.notify_to && !email.test(next.resend.notify_to)) invalid("通知邮箱无效。");
  for (const v of Object.values(next.rate_limits))
    if (!Number.isInteger(v) || v < 1 || v > 100000) invalid("频控必须为 1–100000 的整数。");
  if (
    ![next.spam.review_score, next.spam.block_score].every(
      (n) => Number.isInteger(n) && n >= 0 && n <= 100,
    ) ||
    next.spam.review_score >= next.spam.block_score
  )
    invalid("垃圾信息阈值无效。");
  if (
    !Array.isArray(next.turnstile.hostnames) ||
    next.turnstile.hostnames.some((h) => typeof h !== "string" || !/^[a-z\d.-]+$/i.test(h))
  )
    invalid("Turnstile 主机名无效。");
  for (const n of [next.turnstile.timeout_ms, next.ai_spam.timeout_ms])
    if (!Number.isInteger(n) || n < 1000 || n > 15000) invalid("验证超时必须为 1000–15000 毫秒。");
  if (
    next.turnstile.enabled &&
    (!next.turnstile.site_key ||
      !next.turnstile.hostnames.length ||
      !decryptSecret(next.turnstile.secret_key))
  )
    invalid("请完整配置 Turnstile 与允许的主机名。");
  if (
    !Number.isFinite(next.ai_spam.threshold) ||
    next.ai_spam.threshold < 0 ||
    next.ai_spam.threshold > 1 ||
    !Number.isInteger(next.ai_spam.daily_limit) ||
    next.ai_spam.daily_limit < 1 ||
    next.ai_spam.daily_limit > 100000 ||
    !Number.isInteger(next.ai_spam.max_chars) ||
    next.ai_spam.max_chars < 100 ||
    next.ai_spam.max_chars > 5000
  )
    invalid("AI 参数无效。");
  if (!["chat", "decision"].includes(next.ai_spam.style)) invalid("AI 模式无效。");
  if (
    next.ai_spam.enabled &&
    (!/^[a-f\d]{32}$/i.test(next.ai_spam.account_id) ||
      !next.ai_spam.model.startsWith("@cf/") ||
      !decryptSecret(next.ai_spam.api_token))
  )
    invalid("请完整配置 Workers AI。");
  if (next.resend.enabled && !mailConfiguration(next).configured) invalid("请完整配置 Resend。");
  const changes: Record<string, unknown> = {};
  const before = publicSettings(row),
    after = publicSettings({ ...row, document: next });
  for (const key of Object.keys(next))
    if (
      JSON.stringify(row.document[key as keyof Settings]) !==
      JSON.stringify(next[key as keyof Settings])
    )
      changes[key] = {
        old: before[key as keyof typeof before],
        new: after[key as keyof typeof after],
      };
  // Secret states reveal only whether a key exists, never ciphertext or plaintext.
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
