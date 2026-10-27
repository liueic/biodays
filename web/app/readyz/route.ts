import { check, db } from "@/lib/backend/db";
export const dynamic = "force-dynamic";
export async function GET() {
  if (!process.env.SUPABASE_URL || !process.env.SUPABASE_SECRET_KEY)
    return Response.json({ status: "ok", content: "ready", feedback: "unconfigured" });
  try {
    // Also detect a deployment that has not applied the additive Jev migration.
    const r = await db().from("biodays_feedback").select("spam_probability").limit(1);
    check(r.error);
    return Response.json({ status: "ok", content: "ready", feedback: "ready" });
  } catch {
    return Response.json(
      { status: "degraded", content: "ready", feedback: "unavailable" },
      { status: 503 },
    );
  }
}
