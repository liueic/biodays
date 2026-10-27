import { Resend, type WebhookEventPayload } from "resend";
import { rpc } from "@/lib/backend/db";
export const runtime = "nodejs";
export async function POST(request: Request) {
  const secret = process.env.RESEND_WEBHOOK_SECRET;
  if (!secret) return Response.json({ error: { code: "WEBHOOK_UNAVAILABLE" } }, { status: 503 });
  if (Number(request.headers.get("content-length")) > 65536)
    return new Response(null, { status: 413 });
  const reader = request.body?.getReader();
  if (!reader) return new Response(null, { status: 400 });
  const chunks: Uint8Array[] = [];
  let size = 0;
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    size += value.length;
    if (size > 65536) {
      await reader.cancel();
      return new Response(null, { status: 413 });
    }
    chunks.push(value);
  }
  let event: WebhookEventPayload;
  const id = request.headers.get("svix-id") || "";
  try {
    event = new Resend("webhook-verification-only").webhooks.verify({
      payload: Buffer.concat(chunks).toString(),
      headers: {
        id,
        timestamp: request.headers.get("svix-timestamp") || "",
        signature: request.headers.get("svix-signature") || "",
      },
      webhookSecret: secret,
    });
  } catch {
    return Response.json({ error: { code: "INVALID_SIGNATURE" } }, { status: 400 });
  }
  if (!event.type.startsWith("email.")) return Response.json({ received: true });
  const data = event.data as { email_id?: string; to?: string[] };
  if (!data.email_id) return Response.json({ error: { code: "INVALID_EVENT" } }, { status: 400 });
  try {
    await rpc("webhook", {
      p_id: id,
      p_message_id: data.email_id,
      p_type: event.type,
      p_recipients: data.to ?? [],
    });
    return Response.json({ received: true });
  } catch {
    return Response.json({ error: { code: "SERVICE_UNAVAILABLE" } }, { status: 503 });
  }
}
