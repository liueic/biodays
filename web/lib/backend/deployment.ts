import { HttpError } from "./db";

const env = (name: string) => process.env[name]?.trim() ?? "";
const email = /^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/;
const webURL = (value: string) => {
  try {
    const url = new URL(value);
    return /^https?:$/.test(url.protocol) && !url.username && !url.password;
  } catch {
    return false;
  }
};

export function siteURL() {
  const value =
    env("BIODAYS_SITE_URL") ||
    (env("VERCEL_URL") ? `https://${env("VERCEL_URL")}` : "http://localhost:3000");
  if (!webURL(value)) throw new HttpError(503, "SERVICE_UNAVAILABLE", "站点地址配置无效。");
  return new URL(value).origin;
}

export function resendEnvironment() {
  const apiKey = env("RESEND_API_KEY"),
    from = env("RESEND_FROM"),
    notifyTo = env("BIODAYS_NOTIFY_TO");
  const missing = [
    !apiKey && "RESEND_API_KEY",
    !from && "RESEND_FROM",
    !notifyTo && "BIODAYS_NOTIFY_TO",
  ].filter(Boolean) as string[];
  const invalid = [
    from && (!/^[^\r\n]+$/.test(from) || !email.test(from.match(/<([^>]+)>$/)?.[1] ?? from))
      ? "RESEND_FROM"
      : "",
    notifyTo && !email.test(notifyTo) ? "BIODAYS_NOTIFY_TO" : "",
  ].filter(Boolean);
  return {
    apiKey,
    from,
    notifyTo,
    missing,
    invalid,
    configured: !missing.length && !invalid.length,
  };
}

export function turnstileEnvironment() {
  const siteKey = env("TURNSTILE_SITE_KEY"),
    secretKey = env("TURNSTILE_SECRET_KEY"),
    hostnames = env("TURNSTILE_HOSTNAMES")
      .split(",")
      .map((v) => v.trim().toLowerCase())
      .filter(Boolean);
  const missing = [
    !siteKey && "TURNSTILE_SITE_KEY",
    !secretKey && "TURNSTILE_SECRET_KEY",
    !hostnames.length && "TURNSTILE_HOSTNAMES",
  ].filter(Boolean) as string[];
  const invalid = hostnames.some((h) => !/^[a-z\d.-]+$/i.test(h)) ? ["TURNSTILE_HOSTNAMES"] : [];
  return {
    siteKey,
    secretKey,
    hostnames,
    missing,
    invalid,
    configured: !missing.length && !invalid.length,
  };
}

export const gatewayKey = () => env("AI_GATEWAY_API_KEY");
export type EnvironmentCheck = {
  name: string;
  label: string;
  status: "configured" | "missing" | "invalid" | "default";
  secret: boolean;
  value?: string;
  message?: string;
};

// These checks never send mail, call AI, or expose credentials to the browser.
export function deploymentStatus() {
  const resend = resendEnvironment(),
    turnstile = turnstileEnvironment();
  const checks: EnvironmentCheck[] = [];
  const add = (name: string, label: string, secret = false, valid = true) => {
    const value = env(name);
    checks.push({
      name,
      label,
      secret,
      status: !value ? "missing" : valid ? "configured" : "invalid",
      ...(!secret && value && valid ? { value } : {}),
    });
  };
  let url = "";
  try {
    url = siteURL();
  } catch {
    /* Report invalid configuration without breaking settings. */
  }
  checks.push({
    name: "BIODAYS_SITE_URL",
    label: "站点地址",
    secret: false,
    status: !url ? "invalid" : env("BIODAYS_SITE_URL") ? "configured" : "default",
    ...(url ? { value: url } : {}),
    message: env("BIODAYS_SITE_URL") ? undefined : "自动使用 VERCEL_URL 或本地默认地址",
  });
  add("SUPABASE_URL", "Supabase 项目", false, webURL(env("SUPABASE_URL")));
  add("SUPABASE_PUBLISHABLE_KEY", "Supabase 会话密钥", true);
  add("SUPABASE_SECRET_KEY", "Supabase 服务端密钥", true);
  add("RESEND_API_KEY", "Resend API Key", true);
  add("RESEND_FROM", "发件地址", false, !resend.invalid.includes("RESEND_FROM"));
  add("BIODAYS_NOTIFY_TO", "管理员通知邮箱", false, !resend.invalid.includes("BIODAYS_NOTIFY_TO"));
  add("RESEND_WEBHOOK_SECRET", "Resend Webhook 签名密钥", true);
  add("BIODAYS_CRON_SECRET", "邮件队列调度密钥", true);
  add("AI_GATEWAY_API_KEY", "AI Gateway API Key", true);
  add("TURNSTILE_SITE_KEY", "Turnstile Site Key");
  add("TURNSTILE_SECRET_KEY", "Turnstile Secret Key", true);
  add("TURNSTILE_HOSTNAMES", "Turnstile 允许的域名", false, !turnstile.invalid.length);
  const allowlist = env("BIODAYS_MAIL_ALLOWLIST"),
    preview = env("VERCEL_ENV") === "preview";
  checks.push({
    name: "BIODAYS_MAIL_ALLOWLIST",
    label: "邮件收件人白名单",
    secret: false,
    status: allowlist ? "configured" : "default",
    value: allowlist || (preview ? "未设置：预览环境暂停发送" : "未限制"),
  });
  checks.push({
    name: "BIODAYS_FEEDBACK_ENABLED",
    label: "反馈服务总开关",
    secret: false,
    status: env("BIODAYS_FEEDBACK_ENABLED") ? "configured" : "default",
    value: env("BIODAYS_FEEDBACK_ENABLED") === "false" ? "关闭" : "开启",
  });
  return {
    source: "environment" as const,
    environment: env("VERCEL_ENV") || "local",
    checks,
    services: {
      resend: { configured: resend.configured, missing: resend.missing, invalid: resend.invalid },
      turnstile: {
        configured: turnstile.configured,
        missing: turnstile.missing,
        invalid: turnstile.invalid,
      },
      ai_gateway: {
        configured: !!gatewayKey(),
        missing: gatewayKey() ? [] : ["AI_GATEWAY_API_KEY"],
        invalid: [] as string[],
      },
    },
  };
}
