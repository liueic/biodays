import { createHash } from "node:crypto";
import { type CreateEmailOptions, Resend } from "resend";
import { check, db, newID, rpc } from "./db";

export { siteURL } from "./deployment";

import { mailConfiguration, type Settings, settings } from "./settings";

export type QueueMessage = {
  id: string;
  feedback_id?: string;
  type: string;
  event_key: string;
  recipient: string;
  payload: { subject: string; text: string; html: string; from?: string; to?: string[] };
};
const escapeHTML = (value: string) =>
  value.replace(
    /[&<>"']/g,
    (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c] as string,
  );
export function message(
  type: string,
  eventKey: string,
  recipient: string,
  subject: string,
  text: string,
  feedbackID?: string,
): QueueMessage {
  return {
    id: newID(),
    feedback_id: feedbackID,
    type,
    event_key: eventKey,
    recipient,
    payload: {
      subject,
      text,
      html: `<div style="white-space:pre-wrap">${escapeHTML(text).replace(/https?:\/\/[^\s<>]+/g, (url) => `<a href="${url}">${url}</a>`)}</div>`,
    },
  };
}
export function safeTask(task: Record<string, unknown>) {
  const {
    payload: _p,
    recipient: _r,
    lease_token: _l,
    event_key: _e,
    provider_key_hash: _k,
    ...rest
  } = task;
  return rest;
}
export async function mailStatus(s: Settings) {
  const config = mailConfiguration(s),
    counts: Record<string, number> = {};
  for (const status of ["pending", "sending", "sent", "failed", "unknown", "cancelled"]) {
    const result = await db()
      .from("biodays_outbox")
      .select("id", { count: "exact", head: true })
      .eq("status", status);
    check(result.error);
    counts[status] = result.count ?? 0;
  }
  const recent = await db()
    .from("biodays_outbox")
    .select("*")
    .in("status", ["failed", "unknown"])
    .order("updated_at", { ascending: false })
    .limit(10);
  check(recent.error);
  return {
    resend_enabled: config.enabled,
    configured: config.configured,
    missing: config.missing,
    notify_configured: !!config.notifyTo,
    error: config.error,
    counts,
    recent_errors: (recent.data ?? []).map(safeTask),
  };
}
export function failureStatus(status: number | undefined, firstAttempt: string, now = Date.now()) {
  if (status === 409 || now - new Date(firstAttempt).getTime() > 23 * 3600000) return "unknown";
  if (status === 429 || (status && status >= 500)) return "pending";
  if (status && status >= 400 && status < 500) return "failed";
  // Transport failure: the remote service might have accepted the message.
  return "unknown";
}
type Claimed = QueueMessage & {
  lease_token: string;
  first_attempt_at: string;
  attempts: number;
  provider_key_hash: string;
};
export async function processMail() {
  const started = Date.now();
  const s = (await settings()).document,
    config = mailConfiguration(s);
  await rpc("cleanup");
  if (!config.enabled || !config.configured) return { processed: 0, paused: true };
  const client = new Resend(config.apiKey);
  const keyHash = createHash("sha256").update(config.apiKey).digest("hex");
  let processed = 0;
  // Claim individually: a function nearing its deadline leaves other tasks untouched.
  while (processed < 5 && Date.now() - started < 25000) {
    const tasks = await rpc<Claimed[]>("claim", { p_limit: 1 });
    const task = tasks[0];
    if (!task) break;
    let status = "unknown",
      error = "",
      providerID: string | null = null;
    try {
      if (task.provider_key_hash && task.provider_key_hash !== keyHash)
        throw new Error("Provider key changed after first attempt");
      // Freeze from/to before the first external request. Retries use the exact
      // same payload and stable key even if deployment configuration changes.
      const frozen = {
        ...task.payload,
        from: task.payload.from || config.from,
        to: task.payload.to || [task.recipient],
      };
      const updated = await db()
        .from("biodays_outbox")
        .update({ payload: frozen, provider_key_hash: keyHash })
        .eq("id", task.id)
        .eq("status", "sending")
        .eq("lease_token", task.lease_token)
        .select("id");
      check(updated.error);
      if (!updated.data?.length) continue;
      const allowlist = process.env.BIODAYS_MAIL_ALLOWLIST?.split(",")
        .map((v) => v.trim().toLowerCase())
        .filter(Boolean);
      // Preview deployments fail closed unless an explicit test inbox is set.
      if (
        (process.env.VERCEL_ENV === "preview" && !allowlist?.length) ||
        (allowlist?.length && frozen.to.some((email) => !allowlist.includes(email.toLowerCase())))
      ) {
        status = "failed";
        error = "Recipient blocked by preview/test allowlist";
      } else {
        const result = await client.emails.send(frozen as CreateEmailOptions, {
          idempotencyKey: `biodays/${task.id}`,
          signal: AbortSignal.timeout(8000),
        });
        if (result.error) {
          status = failureStatus(result.error.statusCode ?? undefined, task.first_attempt_at);
          error = `Resend ${result.error.name} (${result.error.statusCode ?? "transport"})`;
        } else if (result.data?.id) {
          status = "sent";
          providerID = result.data.id;
        } else {
          status = "unknown";
          error = "Provider returned no message ID";
        }
      }
    } catch {
      status = "unknown";
      error = "Delivery outcome unknown; reconcile in Resend";
    }
    await rpc("finish", {
      p_id: task.id,
      p_lease: task.lease_token,
      p_status: status,
      p_error: error,
      p_message_id: providerID,
    });
    processed++;
    // Stay below the provider's initial account rate limit.
    await new Promise((resolve) => setTimeout(resolve, 600));
  }
  return { processed, paused: false };
}
