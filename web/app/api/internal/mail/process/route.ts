import { requireBearer } from "@/lib/backend/auth";
import { HttpError } from "@/lib/backend/db";
import { processMail } from "@/lib/backend/mail";
export const runtime = "nodejs";
export const maxDuration = 60;
export async function POST(request: Request) {
  try {
    requireBearer(request, process.env.BIODAYS_CRON_SECRET);
    return Response.json(await processMail(), { headers: { "Cache-Control": "no-store" } });
  } catch (error) {
    return Response.json(
      { error: { code: error instanceof HttpError ? error.code : "INTERNAL_ERROR" } },
      {
        status: error instanceof HttpError ? error.status : 500,
        headers: { "Cache-Control": "no-store" },
      },
    );
  }
}
