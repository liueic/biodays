import { cookies } from "next/headers";
import type { NextRequest } from "next/server";
import { ApiError, type StoryQuery } from "@/lib/api";
import { clientIP, rateLimit } from "@/lib/backend/abuse";
import {
  admin,
  authClient,
  authOrigin,
  csrfFor,
  requireCSRF,
  sameOrigin,
} from "@/lib/backend/auth";
import { check, db, HttpError, newID, rpc } from "@/lib/backend/db";
import { turnstileEnvironment } from "@/lib/backend/deployment";
import {
  getFeedback,
  resend,
  submit,
  subscriptionAction,
  updateFeedback,
} from "@/lib/backend/feedback";
import { mailStatus, message, safeTask } from "@/lib/backend/mail";
import { mailConfiguration, publicSettings, saveSettings, settings } from "@/lib/backend/settings";
import { datasetMeta, listStories, stories, storyByID, tagCounts } from "@/lib/content";
import { KIND_LABELS } from "@/lib/feedback-labels";
import {
  decodeHistory,
  encodeHistory,
  HISTORY_COOKIE,
  randomUnread,
  todayStory,
} from "@/lib/reading";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";
const json = (data: unknown, status = 200) =>
  Response.json(data, {
    status,
    headers: {
      "Cache-Control": "no-store",
      "Referrer-Policy": "no-referrer",
      "X-Robots-Tag": "noindex",
    },
  });
async function readBody(request: Request) {
  if (request.headers.get("content-type")?.split(";")[0]?.trim() !== "application/json")
    throw new HttpError(415, "UNSUPPORTED_MEDIA_TYPE", "需要 application/json。");
  if (Number(request.headers.get("content-length")) > 65536)
    throw new HttpError(413, "PAYLOAD_TOO_LARGE", "请求过大。");
  const reader = request.body?.getReader();
  if (!reader) throw new HttpError(400, "INVALID_PARAM", "缺少请求内容。");
  let size = 0;
  const chunks: Uint8Array[] = [];
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    size += value.length;
    if (size > 65536) {
      await reader.cancel();
      throw new HttpError(413, "PAYLOAD_TOO_LARGE", "请求过大。");
    }
    chunks.push(value);
  }
  try {
    const body = JSON.parse(Buffer.concat(chunks).toString());
    if (!body || typeof body !== "object" || Array.isArray(body)) throw new Error();
    return body as Record<string, unknown>;
  } catch {
    throw new HttpError(400, "INVALID_PARAM", "JSON 格式无效。");
  }
}
function integer(
  search: URLSearchParams,
  key: string,
  defaultValue: number,
  min: number,
  max: number,
) {
  const raw = search.get(key);
  if (raw === null || raw === "") return defaultValue;
  if (!/^\d+$/.test(raw) || Number(raw) < min || Number(raw) > max)
    throw new HttpError(400, "INVALID_PARAM", `${key} 无效。`);
  return Number(raw);
}
function pagination(search: URLSearchParams) {
  return {
    page: integer(search, "page", 1, 1, 1000000),
    pageSize: integer(search, "page_size", 20, 1, 100),
  };
}
function storyQuery(search: URLSearchParams): StoryQuery {
  const month = integer(search, "month", 0, 1, 12),
    day = integer(search, "day", 0, 1, 31);
  if (day && !month) throw new HttpError(400, "INVALID_PARAM", "day requires month");
  const tag = search.get("tag")?.trim() ?? "",
    q = search.get("q")?.trim() ?? "";
  if ([...q].length > 120 || [...tag].length > 40)
    throw new HttpError(400, "INVALID_PARAM", "查询过长。");
  return { ...pagination(search), month, day, tag, q };
}
async function writeHistory(request: NextRequest, ids: string[], id: string) {
  (await cookies()).set(HISTORY_COOKIE, encodeHistory(ids, id), {
    path: "/",
    httpOnly: true,
    sameSite: "lax",
    secure: request.nextUrl.protocol === "https:",
    maxAge: 90 * 86400,
  });
}
async function handle(request: NextRequest, ctx: { params: Promise<{ path: string[] }> }) {
  try {
    const parts = (await ctx.params).path,
      path = parts.join("/"),
      method = request.method,
      search = request.nextUrl.searchParams;
    const seen = decodeHistory(request.cookies.get(HISTORY_COOKIE)?.value);
    if (method === "GET" && path === "today")
      return json(
        todayStory(stories, search.get("tz") || "+08:00", search.get("fallback") || "random", seen),
      );
    if (method === "GET" && path === "stories") {
      const view = search.get("view") || "full";
      if (!["summary", "full"].includes(view))
        throw new HttpError(400, "INVALID_PARAM", "view 无效。");
      return json(listStories(storyQuery(search), view === "full"));
    }
    if (method === "GET" && path === "stories/random") {
      const raw = search.get("exclude") || "",
        exclude = [
          ...new Set(
            raw
              .split(",")
              .map((s) => s.trim())
              .filter(Boolean),
          ),
        ];
      if (raw.length > 2000 || exclude.length > 100)
        throw new HttpError(400, "INVALID_PARAM", "exclude 过长。");
      const result = randomUnread(stories, exclude, seen);
      await writeHistory(request, result.reset ? [] : seen, result.story.id);
      return json(result.story);
    }
    if (method === "GET" && parts[0] === "stories" && parts.length === 2)
      return json(storyByID(parts[1].trim()));
    if (method === "GET" && path === "tags") return json({ data: tagCounts() });
    if (method === "GET" && path === "meta") return json({ data: datasetMeta() });
    if (method === "POST" && path === "history") {
      sameOrigin(request);
      const body = await readBody(request);
      if (typeof body.id !== "string" || body.id.length > 16)
        throw new HttpError(400, "INVALID_PARAM", "valid story id required");
      const story = storyByID(body.id);
      await writeHistory(request, seen, story.id);
      return new Response(null, { status: 204, headers: { "Cache-Control": "no-store" } });
    }
    if (method === "GET" && path === "feedback/options") {
      const enabled =
        !!process.env.SUPABASE_URL &&
        !!process.env.SUPABASE_SECRET_KEY &&
        process.env.BIODAYS_FEEDBACK_ENABLED !== "false";
      const s = enabled ? (await settings()).document : null;
      return json({
        data: {
          enabled,
          subscriptions_enabled: s?.subscriptions_enabled ?? false,
          kinds: Object.entries(KIND_LABELS).map(([value, label]) => ({ value, label })),
          limits: {
            title_min: 1,
            title_max: 80,
            content_min: 10,
            content_max: 5000,
            nickname_max: 40,
            source_url_max: 2048,
          },
          verification: {
            turnstile_enabled: s?.turnstile.enabled ?? false,
            turnstile_site_key: s?.turnstile.enabled ? turnstileEnvironment().siteKey : undefined,
            ai_assisted: s?.ai_spam.enabled ?? false,
          },
        },
      });
    }
    if (parts[0] === "feedback" && method === "POST") {
      sameOrigin(request);
      if (process.env.BIODAYS_FEEDBACK_ENABLED === "false")
        throw new HttpError(503, "FEEDBACK_UNAVAILABLE", "反馈暂未启用。");
      const body = await readBody(request),
        s = (await settings()).document;
      if (path === "feedback") {
        const result = await submit(body, request, s);
        return json({ data: result.data }, result.created ? 201 : 200);
      }
      if (path === "feedback/subscriptions/confirm")
        return json({ data: await subscriptionAction(body, "confirm", request, s) });
      if (path === "feedback/subscriptions/unsubscribe")
        return json({ data: await subscriptionAction(body, "unsubscribe", request, s) });
      if (path === "feedback/subscriptions/resend") {
        try {
          await resend(body, request, s);
        } catch {
          console.error("Confirmation resend deferred");
        }
        return json({ data: { accepted: true } }, 202);
      }
    }
    if (path === "admin/login/github" && method === "POST") {
      sameOrigin(request);
      await rateLimit("login", [clientIP(request)], 10, 50);
      const auth = await authClient();
      const origin = authOrigin(request);
      const { data, error } = await auth.auth.signInWithOAuth({
        provider: "github",
        options: {
          redirectTo: `${origin}/auth/callback`,
          skipBrowserRedirect: true,
        },
      });
      if (error || !data.url)
        throw new HttpError(503, "OAUTH_UNAVAILABLE", "GitHub 登录暂不可用，请稍后重试。");
      return json({ data: { url: data.url } });
    }
    if (path === "admin/login" && method === "POST") {
      sameOrigin(request);
      throw new HttpError(410, "PASSWORD_LOGIN_DISABLED", "请使用 GitHub 账号登录。");
    }
    if (parts[0] === "admin") {
      const session = await admin(),
        actor = session.user.email || session.user.id;
      if (method !== "GET") requireCSRF(request, session.user.id);
      if (path === "admin/me" && method === "GET")
        return json({
          data: {
            username: actor,
            csrf_token: csrfFor(session.user.id),
            expires_at: new Date(Date.now() + 3600000).toISOString(),
          },
        });
      if (path === "admin/logout" && method === "POST") {
        await session.auth.auth.signOut();
        return json({ data: {} });
      }
      if (parts[1] === "stories") {
        if (method !== "GET")
          throw new HttpError(
            410,
            "GIT_PUBLISHING_REQUIRED",
            "文章已改为 Markdown，请通过 Git 发布。",
          );
        if (parts.length === 2) return json(listStories(storyQuery(search)));
        return json({ data: storyByID(parts[2]) });
      }
      if (path === "admin/settings" && method === "GET")
        return json({ data: publicSettings(await settings()) });
      if (path === "admin/settings" && method === "PATCH")
        return json({ data: await saveSettings(await readBody(request), actor) });
      if (path === "admin/settings/audit" && method === "GET") {
        const { page, pageSize } = pagination(search);
        const r = await db()
          .from("biodays_settings_audit")
          .select("*", { count: "exact" })
          .order("id", { ascending: false })
          .range((page - 1) * pageSize, page * pageSize - 1);
        check(r.error);
        return json({ data: r.data, meta: { page, page_size: pageSize, total: r.count } });
      }
      if (path === "admin/feedback" && method === "GET") {
        const { page, pageSize } = pagination(search);
        let query = db()
          .from("biodays_feedback")
          .select(
            "id,kind,title,content,status,subscription_status,story_id,email,nickname,spam_score,spam_verdict,version,created_at,updated_at",
            { count: "exact" },
          );
        for (const key of ["kind", "status"])
          if (search.get(key)) query = query.eq(key, search.get(key));
        const q = search.get("q")?.trim();
        if (q) {
          if (q.length > 120) throw new HttpError(400, "INVALID_PARAM", "查询过长。");
          query = query.ilike("title", `%${q.replace(/[\\%_]/g, "\\$&")}%`);
        }
        const r = await query
          .order("created_at", { ascending: false })
          .range((page - 1) * pageSize, page * pageSize - 1);
        check(r.error);
        return json({
          data: (r.data ?? []).map(({ content, email, nickname, ...f }) => ({
            ...f,
            summary: [...content].slice(0, 120).join(""),
            has_email: !!email,
            has_nickname: !!nickname,
          })),
          meta: { page, page_size: pageSize, total: r.count },
        });
      }
      if (parts[1] === "feedback" && parts.length === 3) {
        if (method === "GET") return json({ data: await getFeedback(parts[2]) });
        if (method === "PATCH")
          return json(await updateFeedback(parts[2], await readBody(request), actor));
        if (method === "DELETE") {
          await rpc("delete", { p_id: parts[2], p_actor: actor });
          return json({ data: {} });
        }
      }
      if (path === "admin/mail/status" && method === "GET")
        return json({ data: await mailStatus((await settings()).document) });
      if (path === "admin/mail/outbox" && method === "GET") {
        const { page, pageSize } = pagination(search);
        let query = db().from("biodays_outbox").select("*", { count: "exact" });
        for (const key of ["status", "type"])
          if (search.get(key)) query = query.eq(key, search.get(key));
        const r = await query
          .order("created_at", { ascending: false })
          .range((page - 1) * pageSize, page * pageSize - 1);
        check(r.error);
        return json({
          data: (r.data ?? []).map(safeTask),
          meta: { page, page_size: pageSize, total: r.count },
        });
      }
      if (path === "admin/mail/test" && method === "POST") {
        const s = (await settings()).document;
        await rateLimit("test_mail", [session.user.id], s.rate_limits.admin_mail_hourly);
        const notifyTo = mailConfiguration(s).notifyTo;
        if (!notifyTo) throw new HttpError(400, "INVALID_PARAM", "请先设置管理员通知邮箱。");
        await rpc("enqueue", {
          p_mail: [
            message(
              "resend_test",
              `test:${newID()}`,
              notifyTo,
              "Biodays Resend 测试",
              "这是一封 Biodays 测试邮件。",
            ),
          ],
        });
        return json({ data: { queued: true } });
      }
      if (
        parts.length === 5 &&
        parts[1] === "mail" &&
        parts[2] === "outbox" &&
        parts[4] === "retry" &&
        method === "POST"
      ) {
        const s = (await settings()).document;
        await rateLimit("retry_mail", [session.user.id], s.rate_limits.admin_mail_hourly);
        await rpc("retry", { p_id: parts[3] });
        return json({ data: {} });
      }
    }
    throw new HttpError(404, "NOT_FOUND", "接口不存在。");
  } catch (error) {
    if (error instanceof HttpError || error instanceof ApiError) {
      const response = json(
        {
          error: {
            code: error.code,
            message: error.message,
            ...(error instanceof HttpError && Object.keys(error.fields).length
              ? { fields: error.fields }
              : {}),
          },
        },
        error.status,
      );
      if (error.status === 429) response.headers.set("Retry-After", "60");
      return response;
    }
    console.error("Biodays request failed", error instanceof Error ? error.name : "unknown");
    return json(
      { error: { code: "INTERNAL_ERROR", message: "服务暂时不可用，请稍后再试。" } },
      500,
    );
  }
}

export { handle as GET, handle as POST, handle as PATCH, handle as DELETE };
