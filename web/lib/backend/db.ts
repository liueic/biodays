import { createClient } from "@supabase/supabase-js";

export class HttpError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields: Record<string, string> = {},
  ) {
    super(message);
  }
}
export function db() {
  const url = process.env.SUPABASE_URL;
  const key = process.env.SUPABASE_SECRET_KEY;
  if (!url || !key)
    throw new HttpError(503, "SERVICE_UNAVAILABLE", "反馈服务尚未配置，请稍后再试。");
  return createClient(url, key, {
    auth: { persistSession: false, autoRefreshToken: false },
    global: {
      fetch: ((input, init) =>
        fetch(input, { ...init, signal: AbortSignal.timeout(5000) })) as typeof fetch,
    },
  });
}
export function check(error: { message: string } | null) {
  if (!error) return;
  const message = error.message;
  if (/VERSION_CONFLICT|IDEMPOTENCY_CONFLICT/.test(message))
    throw new HttpError(
      409,
      /VERSION_CONFLICT/.test(message) ? "CONFLICT" : "IDEMPOTENCY_CONFLICT",
      "数据已更改，请刷新后重试。",
    );
  if (/TOKEN_EXPIRED/.test(message))
    throw new HttpError(410, "TOKEN_EXPIRED", "链接已过期，请重新申请确认邮件。");
  if (/TOKEN_INVALID/.test(message))
    throw new HttpError(400, "INVALID_TOKEN", "链接已失效，请重新申请确认邮件。");
  if (/NOT_FOUND/.test(message)) throw new HttpError(404, "NOT_FOUND", "记录不存在。");
  if (/RECONCILIATION_REQUIRED|MAIL_IN_FLIGHT/.test(message))
    throw new HttpError(
      409,
      "RECONCILIATION_REQUIRED",
      "请先在 Resend 核对发送记录；发送中或超过幂等窗口的任务不能直接重试。",
    );
  // Database errors can contain submitted PII. Never echo them to clients/logs.
  throw new HttpError(503, "SERVICE_UNAVAILABLE", "服务暂时不可用，请稍后重试。");
}
export async function rpc<T = unknown>(
  name: string,
  args: Record<string, unknown> = {},
): Promise<T> {
  const { data, error } = await db().rpc(`biodays_${name}`, args);
  check(error);
  return data as T;
}
export const newID = () => crypto.randomUUID().replaceAll("-", "");
