import { createHash, createHmac } from "node:crypto";
import type { SubmitFeedbackInput } from "@/lib/feedback-api";
import { check, db, HttpError, rpc } from "./db";
import { turnstileEnvironment } from "./deployment";
import { classifyWithJev } from "./jev";
import { jevConfiguration, type Settings } from "./settings";
import { disposableDomains, shortenerHosts, spamKeywords } from "./spam-lists";
export const hash = (value: string) => createHash("sha256").update(value).digest("hex");
export function clientIP(request: Request) {
  // Vercel overwrites this header. A self-hosted proxy must do the same.
  return (
    (process.env.VERCEL
      ? request.headers.get("x-vercel-forwarded-for")
      : request.headers.get("x-forwarded-for")
    )
      ?.split(",")[0]
      ?.trim() || "local"
  );
}
export async function rateLimit(
  scope: string,
  identities: string[],
  hourly: number,
  daily?: number,
  reject = true,
) {
  const secret = await db().from("biodays_secrets").select("abuse_key").eq("id", true).single();
  check(secret.error);
  if (!secret.data) throw new HttpError(503, "SERVICE_UNAVAILABLE", "频控服务尚未初始化。");
  const now = Date.now(),
    buckets = [];
  for (const identity of identities)
    for (const [window, limit] of [
      [3600000, hourly],
      [86400000, daily],
    ] as const) {
      if (!limit) continue;
      const start = Math.floor(now / window) * window;
      buckets.push({
        key: createHmac("sha256", secret.data.abuse_key)
          .update(`${scope}:${window}:${identity}`)
          .digest("hex"),
        start: new Date(start).toISOString(),
        expires: new Date(start + window).toISOString(),
        limit,
      });
    }
  const allowed = await rpc<boolean>("rate", { p_buckets: buckets });
  if (!allowed && reject) throw new HttpError(429, "RATE_LIMITED", "请求过多，请稍后重试。");
  return allowed;
}
export function heuristic(
  input: Pick<SubmitFeedbackInput, "title" | "content" | "source_url" | "email">,
  duplicateCount = 0,
  domainCount = 0,
) {
  let score = 0;
  const signals: string[] = [];
  const add = (weight: number, signal: string) => {
    score += weight;
    signals.push(signal);
  };
  const text = `${input.title}\n${input.content}`,
    links = text.match(/https?:\/\/[^\s<>"']+/gi) ?? [],
    linkCount = links.length + (input.source_url ? 1 : 0);
  if (linkCount > 5) add(35, "many_links");
  else if (linkCount > 2) add(20, "several_links");
  if (
    links.some((url) => {
      try {
        return shortenerHosts.includes(new URL(url).hostname.toLowerCase());
      } catch {
        return false;
      }
    })
  )
    add(30, "shortener_link");
  if (/[\u200b-\u200d\u2060\ufeff]/u.test(text)) add(20, "zero_width");
  if (
    [...text].some(
      (c) =>
        (c.charCodeAt(0) < 32 && ![9, 10, 13].includes(c.charCodeAt(0))) || c.charCodeAt(0) === 127,
    )
  )
    add(30, "control_chars");
  if (duplicateCount >= 1) add(duplicateCount >= 3 ? 60 : 40, "duplicate_content");
  if (domainCount >= 5) add(15, "email_domain_burst");
  if (disposableDomains.includes(input.email?.split("@")[1]?.toLowerCase() ?? ""))
    add(25, "disposable_email");
  if (spamKeywords.some((word) => text.toLowerCase().includes(word))) add(15, "spam_keyword");
  const remainder = text.replace(/https?:\/\/[^\s<>"']+/gi, "").trim();
  if (links.length && remainder.length > 0 && [...remainder].length < 10) add(20, "url_only");
  const letters = [...text].filter((c) => /\p{L}/u.test(c)),
    upper = letters.filter((c) => /\p{Lu}/u.test(c));
  if (letters.length > 20 && upper.length / letters.length > 0.8) add(10, "excessive_caps");
  if (/(.)\1{20}/u.test(text)) add(10, "repeated_chars");
  return { score: Math.min(100, score), signals };
}
export function redact(text: string, maxChars: number) {
  return [
    ...text
      .replace(/https?:\/\/[^\s<>"']+/gi, (url) => {
        try {
          return `[链接:${new URL(url).hostname}]`;
        } catch {
          return "[链接]";
        }
      })
      .replace(/[A-Z\d._%+-]+@[A-Z\d.-]+\.[A-Z]{2,}/gi, "[邮箱]")
      .replace(/(?:\+?86[-\s]?)?1[3-9]\d{9}/g, "[电话]")
      .replace(/[\u200b-\u200d\u2060\ufeff]/gu, ""),
  ]
    .slice(0, maxChars)
    .join("");
}
export async function spamDecision(input: SubmitFeedbackInput, s: Settings) {
  const duplicateHash = hash(`${input.title}\0${input.content}`);
  const duplicates = await db()
    .from("biodays_feedback")
    .select("id", { count: "exact", head: true })
    .eq("content_hash", duplicateHash)
    .gte("created_at", new Date(Date.now() - 30 * 86400000).toISOString());
  check(duplicates.error);
  const domain = input.email?.split("@")[1] || "";
  const domains = domain
    ? await db()
        .from("biodays_feedback")
        .select("id", { count: "exact", head: true })
        .eq("email_domain", domain)
        .gte("created_at", new Date(Date.now() - 3600000).toISOString())
    : null;
  if (domains) check(domains.error);
  const result = s.spam.enabled
    ? heuristic(input, duplicates.count ?? 0, domains?.count ?? 0)
    : { score: 0, signals: [] as string[] };
  const verdict =
    result.score >= s.spam.block_score
      ? "spam"
      : result.score >= s.spam.review_score
        ? "review"
        : "allow";
  const decision = {
    spam_score: result.score,
    spam_verdict: verdict,
    spam_signals: result.signals,
    spam_source: "heuristic",
    spam_model: "",
    spam_confidence: 0,
    spam_probability: null as number | null,
    content_hash: duplicateHash,
    email_domain: domain,
  };
  if (verdict !== "review" || !s.ai_spam.enabled) return decision;
  try {
    if (!jevConfiguration(s.ai_spam).configured)
      return { ...decision, spam_source: "ai_unavailable" };
    if (!(await rateLimit("ai", ["global"], s.ai_spam.daily_limit, s.ai_spam.daily_limit, false)))
      return { ...decision, spam_source: "ai_budget" };
    const classified = await classifyWithJev(
      redact(`${input.title}\n${input.content}`, s.ai_spam.max_chars),
      s.ai_spam,
    );
    return {
      ...decision,
      spam_source: "ai",
      spam_model: classified.model,
      spam_probability: classified.probability,
      spam_verdict:
        classified.probability >= s.ai_spam.threshold
          ? "spam"
          : 1 - classified.probability >= s.ai_spam.threshold
            ? "allow"
            : verdict,
    };
  } catch {
    return { ...decision, spam_source: "ai_error" };
  }
}
export async function verifyTurnstile(token: string | undefined, request: Request, s: Settings) {
  if (!s.turnstile.enabled) return;
  const config = turnstileEnvironment();
  if (!config.configured)
    throw new HttpError(503, "VERIFICATION_UNAVAILABLE", "人机验证环境变量配置不完整。");
  if (!token) throw new HttpError(400, "TURNSTILE_REQUIRED", "请完成人机验证。");
  try {
    const response = await fetch("https://challenges.cloudflare.com/turnstile/v0/siteverify", {
      method: "POST",
      body: new URLSearchParams({
        secret: config.secretKey,
        response: token,
        remoteip: clientIP(request),
      }),
      signal: AbortSignal.timeout(s.turnstile.timeout_ms),
    });
    if (!response.ok) throw new Error("Unavailable");
    const data = await response.json();
    if (!data.success || !config.hostnames.includes(data.hostname) || data.action !== "feedback")
      throw new HttpError(403, "TURNSTILE_FAILED", "人机验证失败，请重试。");
  } catch (e) {
    if (e instanceof HttpError) throw e;
    throw new HttpError(503, "VERIFICATION_UNAVAILABLE", "验证服务暂时不可用。");
  }
}
