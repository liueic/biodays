import { randomBytes } from "node:crypto";
import type { FeedbackDetail } from "@/lib/admin-api";
import type { SubmitFeedbackInput } from "@/lib/feedback-api";
import { feedbackArticle } from "@/lib/feedback-article";
import { KIND_LABELS, STATUS_LABELS } from "@/lib/feedback-labels";
import { clientIP, hash, rateLimit, spamDecision, verifyTurnstile } from "./abuse";
import { check, db, HttpError, newID, rpc } from "./db";
import { message, safeTask, siteURL } from "./mail";
import { mailConfiguration, type Settings } from "./settings";

const length = (s: string) => [...s].length;
export function normalizeSubmission(raw: Record<string, unknown>): SubmitFeedbackInput {
  const fields: Record<string, string> = {};
  const string = (key: string, min: number, max: number) => {
    const value =
      raw[key] === undefined ? "" : typeof raw[key] === "string" ? (raw[key] as string).trim() : "";
    if (
      (raw[key] !== undefined && typeof raw[key] !== "string") ||
      length(value) < min ||
      length(value) > max
    )
      fields[key] = `长度须为 ${min}–${max} 个字符。`;
    return value;
  };
  const request_id = string("request_id", 16, 64),
    kind = string("kind", 1, 16),
    title = string("title", 1, 80),
    content = string("content", 10, 5000),
    nickname = string("nickname", 0, 40),
    email = string("email", 0, 254).toLowerCase(),
    source_url = string("source_url", 0, 2048),
    story_id = string("story_id", 0, 16);
  if (!/^[a-z\d_-]+$/i.test(request_id)) fields.request_id = "请求标识无效。";
  if (!Object.hasOwn(KIND_LABELS, kind)) fields.kind = "请选择反馈类型。";
  if (email && !/^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/.test(email)) fields.email = "邮箱格式无效。";
  if (typeof raw.subscribe_updates !== "boolean") fields.subscribe_updates = "订阅选项无效。";
  if (raw.subscribe_updates === true && !email) fields.email = "订阅需要提供邮箱。";
  if (source_url) {
    try {
      const url = new URL(source_url);
      if (!/^https?:$/.test(url.protocol) || url.username || url.password) throw new Error();
    } catch {
      fields.source_url = "请填写 HTTP(S) 链接。";
    }
  }
  if (story_id && !feedbackArticle(story_id)) fields.story_id = "关联文章不存在。";
  const website = string("website", 0, 1000),
    turnstile_token = string("turnstile_token", 0, 4096);
  if (Object.keys(fields).length)
    throw new HttpError(400, "INVALID_PARAM", "请检查填写内容。", fields);
  return {
    request_id,
    kind: kind as SubmitFeedbackInput["kind"],
    title,
    content,
    nickname,
    email,
    source_url,
    story_id,
    subscribe_updates: raw.subscribe_updates === true,
    website,
    turnstile_token,
  };
}
export function submissionHash(input: SubmitFeedbackInput) {
  const { request_id: _id, website: _w, turnstile_token: _t, ...content } = input;
  return hash(JSON.stringify(content));
}
export function subscriptionMail(
  f: { id: string; email: string; title: string },
  purpose: "confirm" | "unsubscribe",
) {
  const token = randomBytes(32).toString("base64url");
  const record = {
    token_hash: hash(token),
    feedback_id: f.id,
    purpose,
    expires_at: new Date(
      Date.now() + (purpose === "confirm" ? 86400000 : 90 * 86400000),
    ).toISOString(),
  };
  const link = `${siteURL()}/feedback/${purpose === "confirm" ? "confirm" : "unsubscribe"}#token=${token}`;
  return { record, link };
}
const receipt = (f: Record<string, unknown>) => ({
  id: f.id,
  status: f.status,
  subscription_status: f.subscription_status,
  confirmation_email: f.confirmation_email,
  created_at: f.created_at,
});
function articleMailContext(id: string | undefined) {
  if (!id) return "";
  const article = feedbackArticle(id);
  if (!article) return `\n\n关联文章：${id}（暂不可用）`;
  return `\n\n关联文章：${article.title}（${article.id}）\n摘要：${article.summary}\n阅读全文：${siteURL()}${article.path}`;
}
async function allowConfirmation(email: string, s: Settings) {
  if (
    !(await rateLimit(
      "confirmation_send",
      [`email:${email}`],
      s.rate_limits.confirm_hourly,
      s.rate_limits.confirm_daily,
      false,
    ))
  )
    return false;
  return rateLimit(
    "confirmation_send_global",
    ["global"],
    s.rate_limits.confirm_daily_global,
    s.rate_limits.confirm_daily_global,
    false,
  );
}
export async function submit(raw: Record<string, unknown>, request: Request, s: Settings) {
  const input = normalizeSubmission(raw),
    bodyHash = submissionHash(input);
  const prior = await db()
    .from("biodays_feedback")
    .select("*")
    .eq("request_id", input.request_id)
    .maybeSingle();
  check(prior.error);
  if (prior.data) {
    if (prior.data.request_body_hash !== bodyHash)
      throw new HttpError(409, "IDEMPOTENCY_CONFLICT", "请求标识已被用于不同内容。");
    return { data: receipt(prior.data), created: false };
  }
  await rateLimit(
    "submit",
    [clientIP(request), ...(input.email ? [`email:${input.email}`] : [])],
    s.rate_limits.submit_hourly,
    s.rate_limits.submit_daily,
  );
  if (input.website) throw new HttpError(400, "INVALID_PARAM", "提交未通过验证。");
  await verifyTurnstile(input.turnstile_token, request, s);
  const decision = await spamDecision(input, s);
  const subscribed =
    s.subscriptions_enabled && input.subscribe_updates && decision.spam_verdict !== "spam";
  const f = {
    id: newID(),
    kind: input.kind,
    title: input.title,
    content: input.content,
    source_url: input.source_url,
    story_id: input.story_id,
    nickname: input.nickname,
    email: input.email,
    status: decision.spam_verdict === "spam" ? "spam" : "pending",
    subscription_status: subscribed ? "pending_confirmation" : "none",
    confirmation_email: subscribed
      ? mailConfiguration(s).enabled && mailConfiguration(s).configured
        ? "queued"
        : "deferred"
      : "none",
    consent_version: subscribed ? "2026-10-v1" : "",
    consented_at: subscribed ? new Date().toISOString() : null,
    ...decision,
    verification_method: s.turnstile.enabled ? "turnstile" : "none",
    verified_at: s.turnstile.enabled ? new Date().toISOString() : null,
  };
  const canConfirm = subscribed && !!input.email && (await allowConfirmation(input.email, s));
  if (subscribed && !canConfirm) f.confirmation_email = "deferred";
  const tokens = [],
    mail = [];
  if (canConfirm && input.email) {
    const confirmation = subscriptionMail(
      { id: f.id, email: input.email, title: f.title },
      "confirm",
    );
    tokens.push(confirmation.record);
    mail.push(
      message(
        "subscription_confirm",
        `confirm:${f.id}:initial`,
        input.email,
        "确认订阅 Biodays 反馈处理通知",
        `你在 Biodays 请求订阅反馈处理通知。\n请在 24 小时内打开链接并点击确认：\n${confirmation.link}\n\n如非本人操作，请忽略此邮件。`,
        f.id,
      ),
    );
  }
  const notifyTo = mailConfiguration(s).notifyTo;
  if (decision.spam_verdict !== "spam" && notifyTo)
    mail.push(
      message(
        "admin_new_feedback",
        `new:${f.id}`,
        notifyTo,
        `Biodays 新反馈：${f.title}`,
        `${KIND_LABELS[f.kind]}\n${f.title}\n\n${f.content}\n\n来源：${f.source_url || "无"}${articleMailContext(f.story_id)}\n\n处理反馈：${siteURL()}/admin/feedback/${f.id}`,
        f.id,
      ),
    );
  const saved = await rpc<Record<string, unknown>>("submit", {
    p_feedback: f,
    p_request_id: input.request_id,
    p_hash: bodyHash,
    p_tokens: tokens,
    p_mail: mail,
  });
  return { data: receipt(saved), created: saved._created === true };
}
export async function getFeedback(id: string): Promise<FeedbackDetail> {
  const f = await db().from("biodays_feedback").select("*").eq("id", id).maybeSingle();
  check(f.error);
  if (!f.data) throw new HttpError(404, "FEEDBACK_NOT_FOUND", "反馈不存在。");
  const [events, mail] = await Promise.all([
    db()
      .from("biodays_events")
      .select("seq,from_status,to_status,internal_note,public_reply,actor,created_at")
      .eq("feedback_id", id)
      .order("seq"),
    db().from("biodays_outbox").select("*").eq("feedback_id", id).order("created_at"),
  ]);
  check(events.error);
  check(mail.error);
  const {
    request_id: _r,
    request_body_hash: _h,
    content_hash: _c,
    email_domain: _d,
    ...data
  } = f.data;
  return {
    ...data,
    related_article: feedbackArticle(data.story_id),
    email_verified: !!data.confirmed_at,
    events: events.data ?? [],
    mail: (mail.data ?? []).map(safeTask),
  } as FeedbackDetail;
}
export async function updateFeedback(id: string, raw: Record<string, unknown>, actor: string) {
  if (!Number.isInteger(raw.version) || typeof raw.notify_subscriber !== "boolean")
    throw new HttpError(400, "INVALID_PARAM", "缺少版本或通知选项。");
  const patch: Record<string, string> = {};
  if (raw.status !== undefined) {
    if (typeof raw.status !== "string" || !Object.hasOwn(STATUS_LABELS, raw.status))
      throw new HttpError(400, "INVALID_PARAM", "状态无效。");
    patch.status = raw.status;
  }
  for (const key of ["internal_note", "public_reply"])
    if (raw[key] !== undefined) {
      if (typeof raw[key] !== "string" || length(raw[key] as string) > 5000)
        throw new HttpError(400, "INVALID_PARAM", "回复或备注过长。");
      patch[key] = (raw[key] as string).trim();
    }
  const before = await getFeedback(id),
    tokens = [],
    mail = [];
  const notify =
    raw.notify_subscriber &&
    patch.status &&
    patch.status !== before.status &&
    ["accepted", "completed", "rejected"].includes(patch.status) &&
    before.subscription_status === "confirmed";
  if (notify) {
    const unsub = subscriptionMail(before, "unsubscribe");
    tokens.push(unsub.record);
    mail.push(
      message(
        "user_status_update",
        `status:${id}:${Number(raw.version) + 1}`,
        before.email,
        `Biodays 反馈处理结果：${before.title}`,
        `你的反馈「${before.title}」状态已更新为：${STATUS_LABELS[patch.status]}。\n\n${patch.public_reply ?? before.public_reply}${articleMailContext(before.story_id)}\n\n退订通知：${unsub.link}`,
        id,
      ),
    );
  }
  await rpc("update_feedback", {
    p_id: id,
    p_version: raw.version,
    p_patch: patch,
    p_actor: actor,
    p_tokens: tokens,
    p_mail: mail,
  });
  return {
    data: await getFeedback(id),
    meta: {
      notified: !!notify,
      notify_skipped_note: notify
        ? undefined
        : "仅对已确认订阅者的采纳、完成或拒绝状态变更发送通知。",
    },
  };
}
export async function subscriptionAction(
  raw: Record<string, unknown>,
  purpose: "confirm" | "unsubscribe",
  request: Request,
  _s: Settings,
) {
  if (typeof raw.token !== "string" || !/^[\w-]{43}$/.test(raw.token))
    throw new HttpError(400, "INVALID_TOKEN", "链接无效。");
  await rateLimit(purpose, [clientIP(request)], 30, 100);
  return rpc("subscription", { p_hash: hash(raw.token), p_purpose: purpose });
}
export async function resend(raw: Record<string, unknown>, request: Request, s: Settings) {
  if (
    typeof raw.feedback_id !== "string" ||
    typeof raw.email !== "string" ||
    raw.email.length > 254 ||
    raw.feedback_id.length > 64
  )
    throw new HttpError(400, "INVALID_PARAM", "反馈标识或邮箱无效。");
  await rateLimit(
    "resend",
    [clientIP(request), `email:${raw.email.trim().toLowerCase()}`],
    s.rate_limits.resend_hourly,
    s.rate_limits.resend_daily,
  );
  if (!s.subscriptions_enabled) return;
  const f = await db()
    .from("biodays_feedback")
    .select("id,title,email")
    .eq("id", raw.feedback_id)
    .eq("email", raw.email.trim().toLowerCase())
    .maybeSingle();
  check(f.error);
  if (!f.data || !(await allowConfirmation(f.data.email, s))) return;
  const confirmation = subscriptionMail(f.data, "confirm");
  await rpc("resend", {
    p_id: f.data.id,
    p_email: f.data.email,
    p_tokens: [confirmation.record],
    p_mail: [
      message(
        "subscription_confirm",
        `confirm:${newID()}`,
        f.data.email,
        "确认订阅 Biodays 反馈处理通知",
        `你在 Biodays 请求订阅反馈处理通知。\n请在 24 小时内打开链接并点击确认：\n${confirmation.link}\n如非本人操作，请忽略此邮件。`,
        f.data.id,
      ),
    ],
  });
}
