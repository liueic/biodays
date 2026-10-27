import { check, db } from "@/lib/backend/db";
export const dynamic = "force-dynamic";
export async function GET() {
  if (!process.env.SUPABASE_URL || !process.env.SUPABASE_SECRET_KEY)
    return Response.json({ status: "ok", content: "ready", feedback: "unconfigured" });
  try {
    const r = await db().from("biodays_settings").select("id").limit(1);
    check(r.error);
    return Response.json({ status: "ok", content: "ready", feedback: "ready" });
  } catch {
    return Response.json(
      { status: "degraded", content: "ready", feedback: "unavailable" },
      { status: 503 },
    );
  }
}
