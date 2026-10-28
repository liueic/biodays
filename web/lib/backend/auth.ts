import { createHmac, timingSafeEqual } from "node:crypto";
import { createServerClient } from "@supabase/ssr";
import { cookies } from "next/headers";
import { check, db, HttpError } from "./db";

export function sameOrigin(request: Request) {
  const origin = request.headers.get("origin");
  const expected = new URL(process.env.BIODAYS_SITE_URL || request.url).origin;
  if (!origin || origin !== expected || request.headers.get("sec-fetch-site") === "cross-site")
    throw new HttpError(403, "ORIGIN_MISMATCH", "请从本站页面提交。");
}
export async function authClient() {
  const store = await cookies();
  if (!process.env.SUPABASE_URL || !process.env.SUPABASE_PUBLISHABLE_KEY)
    throw new HttpError(503, "SERVICE_UNAVAILABLE", "管理员认证尚未配置。");
  return createServerClient(process.env.SUPABASE_URL, process.env.SUPABASE_PUBLISHABLE_KEY, {
    cookieOptions: {
      httpOnly: true,
      sameSite: "lax",
      secure: process.env.BIODAYS_SITE_URL
        ? new URL(process.env.BIODAYS_SITE_URL).protocol === "https:"
        : process.env.NODE_ENV === "production",
      path: "/",
    },
    cookies: {
      getAll: () => store.getAll(),
      setAll: (values) => {
        for (const { name, value, options } of values) store.set(name, value, options);
      },
    },
  });
}
export async function admin(client?: Awaited<ReturnType<typeof authClient>>) {
  const auth = client ?? (await authClient());
  const { data, error } = await auth.auth.getUser();
  if (error || !data.user) throw new HttpError(401, "UNAUTHORIZED", "请先登录。");
  const membership = await db()
    .from("biodays_admins")
    .select("user_id")
    .eq("user_id", data.user.id)
    .maybeSingle();
  check(membership.error);
  if (!membership.data) throw new HttpError(403, "FORBIDDEN", "此账号没有管理员权限。");
  return { user: data.user, auth };
}
export function requireBearer(request: Request, secret: string | undefined) {
  const provided = request.headers.get("authorization") ?? "";
  const expected = `Bearer ${secret ?? ""}`;
  if (
    !secret ||
    Buffer.byteLength(provided) !== Buffer.byteLength(expected) ||
    !timingSafeEqual(Buffer.from(provided), Buffer.from(expected))
  )
    throw new HttpError(401, "UNAUTHORIZED", "Unauthorized");
}
export const csrfFor = (userID: string) =>
  createHmac("sha256", process.env.SUPABASE_SECRET_KEY ?? "")
    .update(`csrf:${userID}`)
    .digest("hex");
export function requireCSRF(request: Request, userID: string) {
  sameOrigin(request);
  const provided = request.headers.get("x-csrf-token") ?? "",
    expected = csrfFor(userID);
  if (
    Buffer.byteLength(provided) !== Buffer.byteLength(expected) ||
    !timingSafeEqual(Buffer.from(provided), Buffer.from(expected))
  )
    throw new HttpError(403, "CSRF_INVALID", "会话校验失败，请刷新页面。");
}
