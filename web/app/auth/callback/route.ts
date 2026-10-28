import { NextResponse } from "next/server";
import { admin, authClient } from "@/lib/backend/auth";
import { HttpError } from "@/lib/backend/db";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET(request: Request) {
  const url = new URL(request.url);
  const origin = new URL(process.env.BIODAYS_SITE_URL || request.url).origin;
  const redirect = (path: string) =>
    NextResponse.redirect(new URL(path, origin), {
      status: 303,
      headers: {
        "Cache-Control": "no-store",
        "Referrer-Policy": "no-referrer",
        "X-Robots-Tag": "noindex, nofollow",
      },
    });
  const code = url.searchParams.get("code");
  if (!code || url.searchParams.has("error")) return redirect("/admin/login?error=github_failed");

  let auth: Awaited<ReturnType<typeof authClient>> | undefined;
  try {
    auth = await authClient();
    const { error } = await auth.auth.exchangeCodeForSession(code);
    if (error) throw error;
    // Authentication alone never grants administrator membership.
    await admin(auth);
    return redirect("/admin/feedback");
  } catch (error) {
    if (auth) {
      try {
        await auth.auth.signOut({ scope: "local" });
      } catch {
        // Keep provider errors and credentials out of the redirect and logs.
      }
    }
    const reason =
      error instanceof HttpError && error.status === 403
        ? "not_admin"
        : error instanceof HttpError && error.status === 503
          ? "auth_unavailable"
          : "github_failed";
    return redirect(`/admin/login?error=${reason}`);
  }
}
